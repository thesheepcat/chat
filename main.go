package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	ma "github.com/multiformats/go-multiaddr"

	ds "github.com/ipfs/go-datastore"
	dsync "github.com/ipfs/go-datastore/sync"

	rhost "github.com/libp2p/go-libp2p/p2p/host/routed"
)

// DiscoveryInterval is how often we re-publish our mDNS records.
const DiscoveryInterval = time.Hour

// DiscoveryServiceTag is used in our mDNS advertisements to discover other chat peers.
const DiscoveryServiceTag = "pubsub-chat-example"

func main() {
	// parse some flags to set our nickname and the room to join
	nickFlag := flag.String("nick", "", "nickname to use in chat. will be generated if empty")
	roomFlag := flag.String("room", "decentralized-chat", "name of chat room to join")
	// parse flags for bootstrap
	listenF := flag.Int("l", 0, "wait for incoming connections")
	bootstrapPeer := flag.String("b", "", "bootstrap peer")
	flag.Parse()

	if *listenF == 0 {
		log.Fatal("Please provide a port to bind on with -l")
	}

	// Make routed host
	var bootstrapPeers []peer.AddrInfo
	if *bootstrapPeer != "" {
		bootstrapPeers = convertPeers([]string{*bootstrapPeer})
	}
	ha, _, err := makeRoutedHost(*listenF, bootstrapPeers)
	if err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()

	/*
		// create a new libp2p Host that listens on a random TCP port
		h, err := libp2p.New(libp2p.ListenAddrStrings("/ip4/0.0.0.0/tcp/0"))
		if err != nil {
			panic(err)
		}
	*/

	// Defines the default gossipsub parameters.
	var (
		GossipSubD                                = 6
		GossipSubDlo                              = 5
		GossipSubDhi                              = 12
		GossipSubDscore                           = 4
		GossipSubDout                             = 2
		GossipSubHistoryLength                    = 5
		GossipSubHistoryGossip                    = 3
		GossipSubDlazy                            = 6
		GossipSubGossipFactor                     = 0.25
		GossipSubGossipRetransmission             = 3
		GossipSubHeartbeatInitialDelay            = 100 * time.Millisecond
		GossipSubHeartbeatInterval                = 1 * time.Second
		GossipSubFanoutTTL                        = 60 * time.Second
		GossipSubPrunePeers                       = 16
		GossipSubPruneBackoff                     = time.Minute
		GossipSubUnsubscribeBackoff               = 10 * time.Second
		GossipSubConnectors                       = 8
		GossipSubMaxPendingConnections            = 128
		GossipSubConnectionTimeout                = 30 * time.Second
		GossipSubDirectConnectTicks        uint64 = 300
		GossipSubDirectConnectInitialDelay        = time.Second
		GossipSubOpportunisticGraftTicks   uint64 = 60
		GossipSubOpportunisticGraftPeers          = 2
		GossipSubGraftFloodThreshold              = 10 * time.Second
		GossipSubMaxIHaveLength                   = 5000
		GossipSubMaxIHaveMessages                 = 10
		GossipSubIWantFollowupTime                = 3 * time.Second
	)

	// Set customized configuration parameters
	cfg := pubsub.GossipSubParams{
		D:                         GossipSubD,
		Dlo:                       GossipSubDlo,
		Dhi:                       GossipSubDhi,
		Dscore:                    GossipSubDscore,
		Dout:                      GossipSubDout,
		HistoryLength:             GossipSubHistoryLength,
		HistoryGossip:             GossipSubHistoryGossip,
		Dlazy:                     GossipSubDlazy,
		GossipFactor:              GossipSubGossipFactor,
		GossipRetransmission:      GossipSubGossipRetransmission,
		HeartbeatInitialDelay:     GossipSubHeartbeatInitialDelay,
		HeartbeatInterval:         GossipSubHeartbeatInterval,
		FanoutTTL:                 GossipSubFanoutTTL,
		PrunePeers:                GossipSubPrunePeers,
		PruneBackoff:              GossipSubPruneBackoff,
		UnsubscribeBackoff:        GossipSubUnsubscribeBackoff,
		Connectors:                GossipSubConnectors,
		MaxPendingConnections:     GossipSubMaxPendingConnections,
		ConnectionTimeout:         GossipSubConnectionTimeout,
		DirectConnectTicks:        GossipSubDirectConnectTicks,
		DirectConnectInitialDelay: GossipSubDirectConnectInitialDelay,
		OpportunisticGraftTicks:   GossipSubOpportunisticGraftTicks,
		OpportunisticGraftPeers:   GossipSubOpportunisticGraftPeers,
		GraftFloodThreshold:       GossipSubGraftFloodThreshold,
		MaxIHaveLength:            GossipSubMaxIHaveLength,
		MaxIHaveMessages:          GossipSubMaxIHaveMessages,
		IWantFollowupTime:         GossipSubIWantFollowupTime,
		SlowHeartbeatWarning:      0.1,
	}

	// set gossipSub options
	//pubsub.GossipSubHeartbeatInterval = 1000 * time.Millisecond
	options := []pubsub.Option{
		// Gossipsubv1.1 configuration
		//pubsub.WithFloodPublish(true),
		//  buffer, 32 -> 10K
		//pubsub.WithValidateQueueSize(10 << 10),
		//  worker, 1x cpu -> 2x cpu
		//pubsub.WithValidateWorkers(runtime.NumCPU() * 2),
		//  goroutine, 8K -> 16K
		//pubsub.WithValidateThrottle(16 << 10),
		//pubsub.WithMessageSigning(true),
		pubsub.WithPeerExchange(true),
		pubsub.WithGossipSubParams(cfg),
	}

	// create a new PubSub service using the GossipSub router
	ps, err := pubsub.NewGossipSub(ctx, ha, options...)
	if err != nil {
		panic(err)
	}

	/*
		// setup local mDNS discovery
		if err := setupDiscovery(h); err != nil {
			panic(err)
		}
	*/

	// use the nickname from the cli flag, or a default if blank
	nick := *nickFlag
	if len(nick) == 0 {
		nick = defaultNick(ha.ID())
	}

	// join the room from the cli flag, or the flag default
	room := *roomFlag

	// join the chat room
	cr, err := JoinChatRoom(ctx, ps, ha.ID(), nick, room)
	if err != nil {
		panic(err)
	}

	time.Sleep(10 * time.Second)

	// draw the UI
	ui := NewChatUI(cr)
	if err = ui.Run(); err != nil {
		printErr("error running text UI: %s", err)
	}
}

// printErr is like fmt.Printf, but writes to stderr.
func printErr(m string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, m, args...)
}

// defaultNick generates a nickname based on the $USER environment variable and
// the last 8 chars of a peer ID.
func defaultNick(p peer.ID) string {
	return fmt.Sprintf("%s-%s", os.Getenv("USER"), shortID(p))
}

// shortID returns the last 8 chars of a base58-encoded peer id.
func shortID(p peer.ID) string {
	pretty := p.Pretty()
	return pretty[len(pretty)-8:]
}

/*
// discoveryNotifee gets notified when we find a new peer via mDNS discovery
type discoveryNotifee struct {
	h host.Host
}

// HandlePeerFound connects to peers discovered via mDNS. Once they're connected,
// the PubSub system will automatically start interacting with them if they also
// support PubSub.
func (n *discoveryNotifee) HandlePeerFound(pi peer.AddrInfo) {
	fmt.Printf("discovered new peer %s\n", pi.ID.Pretty())
	err := n.h.Connect(context.Background(), pi)
	if err != nil {
		fmt.Printf("error connecting to peer %s: %s\n", pi.ID.Pretty(), err)
	}
}

// setupDiscovery creates an mDNS discovery service and attaches it to the libp2p Host.
// This lets us automatically discover peers on the same LAN and connect to them.
func setupDiscovery(h host.Host) error {
	// setup mDNS discovery to find local peers
	s := mdns.NewMdnsService(h, DiscoveryServiceTag, &discoveryNotifee{h: h})
	return s.Start()
}
*/

// convert peer string into peer.AddrInfo object (es. /ip4/127.0.0.1/tcp/9000/p2p/QmZqyJRZxC1ZZZ66MWeHmBUu97Dt5Q8Jc1ukv6FUgFGv1h)
func convertPeers(peers []string) []peer.AddrInfo {
	pinfos := make([]peer.AddrInfo, len(peers))
	for i, addr := range peers {
		maddr := ma.StringCast(addr)
		p, err := peer.AddrInfoFromP2pAddr(maddr)
		if err != nil {
			log.Fatalln(err)
		}
		pinfos[i] = *p
	}
	return pinfos
}

// Creates a LibP2P host with a random peer ID
func makeRoutedHost(listenPort int, bootstrapPeers []peer.AddrInfo) (host.Host, *dht.IpfsDHT, error) {

	// Generate a key pair for this host
	priv, _, err := crypto.GenerateKeyPair(crypto.RSA, 2048)
	if err != nil {
		return nil, nil, err
	}

	ctx := context.Background()

	opts := []libp2p.Option{
		libp2p.ListenAddrStrings(fmt.Sprintf("/ip4/0.0.0.0/tcp/%d", listenPort)),
		libp2p.Identity(priv),
		libp2p.DefaultTransports,
		libp2p.DefaultMuxers,
		libp2p.DefaultSecurity,
		libp2p.NATPortMap(),
	}

	basicHost, err := libp2p.New(opts...)
	if err != nil {
		return nil, nil, err
	}

	// Construct a datastore
	// TOFIX not sure it is needed
	dstore := dsync.MutexWrap(ds.NewMapDatastore())

	// Make the DHT
	//ndht := dht.NewDHT(ctx, basicHost, dstore)

	ndht, err := dht.New(ctx, basicHost, dht.Datastore(dstore), dht.Mode(dht.ModeServer))
	if err != nil {
		return nil, nil, err
	}
	// Make the routed host
	routedHost := rhost.Wrap(basicHost, ndht)

	// Connect to bootstrap nodes
	if len(bootstrapPeers) > 0 {
		err = bootstrapConnect(ctx, routedHost, bootstrapPeers)
		if err != nil {
			return nil, nil, err
		}
	}

	// Bootstrap the host
	// TOFIX not sure it is needed
	err = ndht.Bootstrap(ctx)
	if err != nil {
		return nil, nil, err
	}

	// Build host multiaddress
	hostAddr, _ := ma.NewMultiaddr(fmt.Sprintf("/p2p/%s", routedHost.ID().Pretty()))

	addrs := routedHost.Addrs()
	log.Println("My ID is :", routedHost.ID().Pretty())
	log.Println("I can be reached at:")
	for _, addr := range addrs {
		log.Println(addr.Encapsulate(hostAddr))
	}

	return routedHost, ndht, nil
}

// This code is borrowed from the go-ipfs bootstrap process
func bootstrapConnect(ctx context.Context, ph host.Host, peers []peer.AddrInfo) error {
	if len(peers) < 1 {
		return errors.New("not enough bootstrap peers")
	}

	errs := make(chan error, len(peers))
	var wg sync.WaitGroup
	for _, p := range peers {

		// performed asynchronously because when performed synchronously, if
		// one `Connect` call hangs, subsequent calls are more likely to
		// fail/abort due to an expiring context.
		// Also, performed asynchronously for dial speed.

		wg.Add(1)
		go func(p peer.AddrInfo) {
			defer wg.Done()
			defer log.Println(ctx, "bootstrapDial", ph.ID(), p.ID)
			log.Printf("%s bootstrapping to %s", ph.ID(), p.ID)

			ph.Peerstore().AddAddrs(p.ID, p.Addrs, 24*time.Hour)
			if err := ph.Connect(ctx, p); err != nil {
				log.Println(ctx, "bootstrapDialFailed", p.ID)
				log.Printf("failed to bootstrap with %v: %s", p.ID, err)
				errs <- err
				return
			}
			log.Println(ctx, "bootstrapDialSuccess", p.ID)
			log.Printf("bootstrapped with %v", p.ID)
		}(p)
	}
	wg.Wait()

	// our failure condition is when no connection attempt succeeded.
	// So drain the errs channel, counting the results.
	close(errs)
	count := 0
	var err error
	for err = range errs {
		if err != nil {
			count++
		}
	}
	if count == len(peers) {
		return fmt.Errorf("failed to bootstrap. %s", err)
	}
	return nil
}
