// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package whatsmeow

import (
	"sync"
	"sync/atomic"
	"time"

	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"
)

type QRChannelItem string

// IsQR returns true if this channel item is an actual QR code rather than a close event.
func (qrci QRChannelItem) IsQR() bool {
	return qrci != QRChannelSuccess && qrci != QRChannelTimeout && qrci != QRChannelErrUnexpectedEvent
}

const (
	// QRChannelSuccess is emitted from GetQRChannel when the pairing is successful.
	QRChannelSuccess QRChannelItem = "success"
	// QRChannelTimeout is emitted from GetQRChannel if the socket gets disconnected by the server before the pairing is successful.
	QRChannelTimeout QRChannelItem = "timeout"
	// QRChannelErrUnexpectedEvent is emitted from GetQRChannel if n unexpected connection event is received,
	// as that likely means that the pairing has already happened before the channel was set up.
	QRChannelErrUnexpectedEvent QRChannelItem = "err-unexpected-state"
)

type qrChannel struct {
	sync.Mutex
	cli       *Client
	log       waLog.Logger
	handlerID uint32
	closed    uint32
	output    chan<- QRChannelItem
	stopQRs   chan struct{}
}

func (qrc *qrChannel) emitQRs(evt *events.QR) {
	var nextCode string
	for {
		if len(evt.Codes) == 0 {
			if atomic.CompareAndSwapUint32(&qrc.closed, 0, 1) {
				qrc.log.Debugf("Ran out of QR codes, closing channel with status %s and disconnecting client", QRChannelTimeout)
				qrc.output <- QRChannelTimeout
				close(qrc.output)
				qrc.cli.Disconnect()
			} else {
				qrc.log.Debugf("Ran out of QR codes, but channel is already closed")
			}
			return
		}
		nextCode, evt.Codes = evt.Codes[0], evt.Codes[1:]
		qrc.log.Debugf("Emitting QR code %s", nextCode)
		select {
		case qrc.output <- QRChannelItem(nextCode):
		default:
			qrc.log.Debugf("Output channel didn't accept code, exiting QR emitter")
			return
		}
		select {
		case <-time.After(evt.Timeout):
		case <-qrc.stopQRs:
			qrc.log.Debugf("Got signal to stop QR emitter")
			return
		}
	}
}

func (qrc *qrChannel) handleEvent(rawEvt interface{}) {
	var outputType QRChannelItem
	switch evt := rawEvt.(type) {
	case *events.QR:
		qrc.log.Debugf("Received QR code event, starting to emit codes to channel")
		go qrc.emitQRs(evt)
		return
	case *events.PairSuccess:
		outputType = QRChannelSuccess
	case *events.Disconnected:
		outputType = QRChannelTimeout
	case *events.Connected, *events.ConnectFailure:
		outputType = QRChannelErrUnexpectedEvent
	default:
		return
	}
	close(qrc.stopQRs)
	if atomic.CompareAndSwapUint32(&qrc.closed, 0, 1) {
		qrc.log.Debugf("Closing channel with status %s", outputType)
		qrc.output <- outputType
		close(qrc.output)
	} else {
		qrc.log.Debugf("Got status %s, but channel is already closed", outputType)
	}
	qrc.cli.RemoveEventHandler(qrc.handlerID)
}

// GetQRChannel returns a channel that automatically outputs a new QR code when the previous one expires.
//
// This must be called *before* Connect(). It will then listen to all the relevant events from the client.
//
// The last value to be emitted will be a special string, either "success", "timeout" or "err-already-have-id",
// depending on the result of the pairing. The channel will be closed immediately after one of those.
func (cli *Client) GetQRChannel() (<-chan QRChannelItem, error) {
	if cli.IsConnected() {
		return nil, ErrQRAlreadyConnected
	} else if cli.Store.ID != nil {
		return nil, ErrQRStoreContainsID
	}
	ch := make(chan QRChannelItem, 7)
	qrc := qrChannel{
		output:  ch,
		stopQRs: make(chan struct{}),
		cli:     cli,
		log:     cli.Log.Sub("QRChannel"),
	}
	qrc.handlerID = cli.AddEventHandler(qrc.handleEvent)
	return ch, nil
}