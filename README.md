# :sailboat: EdgeVPN

Fully Decentralized. Immutable. Portable. Easy to use Statically compiled VPN

## Usage

Generate a config:

```bash
./edgevpn -g > config.yaml
```

Run it on multiple hosts:

```bash
EDGEVPNCONFIG=config.yaml IFACE=edgevpn0 ADDRESS=10.1.0.11/24 ./edgevpn
EDGEVPNCONFIG=config.yaml IFACE=edgevpn0 ADDRESS=10.1.0.12/24 ./edgevpn
EDGEVPNCONFIG=config.yaml IFACE=edgevpn0 ADDRESS=10.1.0.13/24 ./edgevpn
...
```

... and that's it!

*Note*: It might take up time to build the connection between nodes. Wait at least 5 mins, it depends on the network behind the hosts.

## Is it for me?

EdgeVPN makes VPN decentralization a first strong requirement. 

Its mainly use is for edge and low-end devices and especially for development.

The decentralized approach has few cons:

- The underlaying network is chatty. It uses a Gossip protocol and p2p. Every message is broadcasted to all peers.
- Not suited for low latency. On my local tests on very slow connections, ping took ~200ms.

Keep that in mind before using it for your prod networks!

But it has a strong pro: it just works everywhere libp2p works!

### Example use case: network-decentralized [k3s](https://github.com/k3s-io/k3s) test cluster

Let's see a practical example, you are developing something for kubernetes and you want to try a multi-node setup, but you have machines available that are only behind NAT (pity!) and you would really like to leverage HW.

If you are not really interested in network performance (again, that's for development purposes only!) then you could use `edgevpn` + [k3s](https://github.com/k3s-io/k3s) in this way:

1) Generate edgevpn config: `edgevpn -g > vpn.yaml`
2) Start the vpn:

   on node A: `sudo IFACE=edgevpn0 ADDRESS=10.1.0.3/24 EDGEVPNCONFIG=vpn.yml edgevpn`
   
   on node B: `sudo IFACE=edgevpn0 ADDRESS=10.1.0.4/24 EDGEVPNCONFIG=vpm.yml edgevpn`
3) Start k3s:
 
   on node A: `k3s server --flannel-iface=edgevpn0`
   
   on node B: `K3S_URL=https://10.1.0.3:6443 K3S_TOKEN=xx k3s agent --flannel-iface=edgevpn0 --node-ip 10.1.0.4`

We have used flannel here, but other CNI should work as well.

## As a library

EdgeVPN can be used as a library. It is very portable and offers a functional interface:

```golang

import (
    edgevpn "github.com/mudler/edgevpn/pkg/edgevpn"
)

e := edgevpn.New(edgevpn.Logger(l),
    edgevpn.LogLevel(log.LevelInfo),
    edgevpn.MaxMessageSize(2 << 20),
    edgevpn.WithMTU(1500),
    edgevpn.WithInterfaceMTU(1300),
    edgevpn.WithInterfaceAddress(os.Getenv("ADDRESS")),
    edgevpn.WithInterfaceName(os.Getenv("IFACE")),
    // ....
    edgevpn.WithInterfaceType(water.TAP))

e.Start()

```

## Architecture

- p2p encryption between peers with libp2p
- randezvous points dynamically generated from OTP keys
- extra AES symmetric encryption on top. In case randezvous point is compromised

## Credits

- The awesome [libp2p](https://github.com/libp2p) library
- [https://github.com/songgao/water](https://github.com/songgao/water) for tun/tap devices in golang
- [Room example](https://github.com/libp2p/go-libp2p/tree/master/examples/chat-with-rendezvous) (shamelessly parts are copied by)

## Disclaimers

I'm not a security expert, and this software didn't went through a full security audit, so don't use and rely it for sensible traffic! I did this mostly for fun while I was experimenting with libp2p. 

## LICENSE

GNU GPLv3.

```
edgevpn  Copyright (C) 2021 Ettore Di Giacinto
This program comes with ABSOLUTELY NO WARRANTY.
This is free software, and you are welcome to redistribute it
under certain conditions.
```