// Copyright © 2021 Ettore Di Giacinto <mudler@mocaccino.org>
//
// This program is free software; you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation; either version 2 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License along
// with this program; if not, see <http://www.gnu.org/licenses/>.

package edgevpn

import (
	"context"
	"io"
	"net"
	"time"

	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/protocol"
	"github.com/mudler/edgevpn/pkg/blockchain"
	"github.com/mudler/edgevpn/pkg/edgevpn/types"
)

const (
	ServicesLedgerKey = "services"
	UsersLedgerKey    = "users"
)

func (e *EdgeVPN) ExposeService(ledger *blockchain.Ledger, serviceID, dstaddress string) {

	e.Logger().Infof("Exposing service '%s' (%s)", serviceID, dstaddress)

	// 1) Register the ServiceID <-> PeerID Association
	// By announcing periodically our service to the blockchain
	ledger.Announce(
		context.Background(),
		e.config.LedgerAnnounceTime,
		func() {
			// Retrieve current ID for ip in the blockchain
			existingValue, found := ledger.GetKey(ServicesLedgerKey, serviceID)
			service := &types.Service{}
			existingValue.Unmarshal(service)
			// If mismatch, update the blockchain
			if !found || service.PeerID != e.host.ID().String() {
				updatedMap := map[string]interface{}{}
				updatedMap[serviceID] = types.Service{PeerID: e.host.ID().String(), Name: serviceID}
				ledger.Add(ServicesLedgerKey, updatedMap)
			}
		},
	)

	// 2) Set a stream handler
	//    which connect to the given address/Port and Send what we receive from the Stream.
	e.config.StreamHandlers[protocol.ID(ServiceProtocol)] = func(stream network.Stream) {
		go func() {
			e.config.Logger.Infof("(service %s) Received connection from %s", serviceID, stream.Conn().RemotePeer().String())

			// Retrieve current ID for ip in the blockchain
			_, found := ledger.GetKey(UsersLedgerKey, stream.Conn().RemotePeer().String())
			// If mismatch, update the blockchain
			if !found {
				e.config.Logger.Debugf("Reset '%s': not found in the ledger", stream.Conn().RemotePeer().String())
				stream.Reset()
				return
			}

			e.config.Logger.Infof("Connecting to '%s'", dstaddress)
			c, err := net.Dial("tcp", dstaddress)
			if err != nil {
				e.config.Logger.Debugf("Reset %s: %s", stream.Conn().RemotePeer().String(), err.Error())
				stream.Reset()
				return
			}
			closer := make(chan struct{}, 2)
			go copyStream(closer, stream, c)
			go copyStream(closer, c, stream)
			<-closer

			stream.Close()
			c.Close()

			e.config.Logger.Infof("(service %s) Handled correctly '%s'", serviceID, stream.Conn().RemotePeer().String())
		}()
	}
}

func (e *EdgeVPN) ConnectToService(ledger *blockchain.Ledger, serviceID string, srcaddr string) error {

	// Open local port for listening
	l, err := net.Listen("tcp", srcaddr)
	if err != nil {
		return err
	}
	e.Logger().Info("Binding local port on", srcaddr)

	// Announce ourselves so nodes accepts our connection
	ledger.Announce(
		context.Background(),
		e.config.LedgerAnnounceTime,
		func() {
			// Retrieve current ID for ip in the blockchain
			_, found := ledger.GetKey(UsersLedgerKey, e.host.ID().String())
			// If mismatch, update the blockchain
			if !found {
				updatedMap := map[string]interface{}{}
				updatedMap[e.host.ID().String()] = &types.User{
					PeerID:    e.host.ID().String(),
					Timestamp: time.Now().String(),
				}
				ledger.Add(UsersLedgerKey, updatedMap)
			}
		},
	)
	defer l.Close()
	for {
		// Listen for an incoming connection.
		conn, err := l.Accept()
		if err != nil {
			e.config.Logger.Error("Error accepting: ", err.Error())
			continue
		}

		e.config.Logger.Info("New connection from", l.Addr().String())
		// Handle connections in a new goroutine, forwarding to the p2p service
		go func() {
			// Retrieve current ID for ip in the blockchain
			existingValue, found := ledger.GetKey(ServicesLedgerKey, serviceID)
			service := &types.Service{}
			existingValue.Unmarshal(service)
			// If mismatch, update the blockchain
			if !found {
				conn.Close()
				e.config.Logger.Debugf("service '%s' not found on blockchain", serviceID)
				return
			}

			// Decode the Peer
			d, err := peer.Decode(service.PeerID)
			if err != nil {
				conn.Close()
				e.config.Logger.Debugf("could not decode peer '%s'", service.PeerID)
				return
			}

			// Open a stream
			stream, err := e.host.NewStream(context.Background(), d, ServiceProtocol)
			if err != nil {
				conn.Close()
				e.config.Logger.Debugf("could not open stream '%s'", err.Error())
				return
			}
			e.config.Logger.Debugf("(service %s) Redirecting", serviceID, l.Addr().String())

			closer := make(chan struct{}, 2)
			go copyStream(closer, stream, conn)
			go copyStream(closer, conn, stream)
			<-closer

			stream.Close()
			conn.Close()
			e.config.Logger.Infof("(service %s) Done handling %s", serviceID, l.Addr().String())
		}()
	}
}

func copyStream(closer chan struct{}, dst io.Writer, src io.Reader) {
	_, _ = io.Copy(dst, src)
	closer <- struct{}{} // connection is closed, send signal to stop proxy
}
