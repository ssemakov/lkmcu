package livekit

import (
	"errors"
	"sync/atomic"

	lksdk "github.com/livekit/server-sdk-go/v2"
	"github.com/pion/webrtc/v4"
)

type Subscription struct {
	TID string
	PID string
	Kind string
	Source string

	// layer int // video quality layer
}

type Client struct {
	apiKey    string
	apiSecret string
	url       string
	room      *lksdk.Room

	evCh chan Event
	dropPolicy DropPolicy
	listSubs func(*lksdk.Room) []*Subscription
	closed atomic.Bool
	// mu sync.Mutex
}

type EventOptions struct {
	BufferSize int // size of the internal event channel buffer
	DropPolicy DropPolicy
}
// Defines how the client should behave when the event buffer (evCh) is full
type DropPolicy int

const (
	DropOldest DropPolicy = iota // drop oldest event when buffer is full
	Block                        // block until there is space in the buffer
)

func NewClient(url, apiKey, apiSecret string, opts *EventOptions) *Client {
	if opts == nil {
		opts = &EventOptions{ BufferSize: 64, DropPolicy: DropOldest }
	}
	return &Client{url: url, apiKey: apiKey, apiSecret: apiSecret,
		evCh: make(chan Event, opts.BufferSize),
		dropPolicy: opts.DropPolicy,
		listSubs: defaultListSubs, }
}

func (c *Client) JoinRoom(roomName, identity string) error {
	roomCB := &lksdk.RoomCallback{
		OnParticipantConnected: func(rp *lksdk.RemoteParticipant) { c.onParticipantJoined(rp) },
		OnParticipantDisconnected: func(rp *lksdk.RemoteParticipant) { c.onParticipantLeft(rp) },
		OnDisconnected: func() { c.onRoomDisconnected(lksdk.OtherReason) },
		OnDisconnectedWithReason: c.onRoomDisconnected,
		OnReconnected: c.onRoomReconnected,

		ParticipantCallback: lksdk.ParticipantCallback{
			OnTrackPublished: func(pub *lksdk.RemoteTrackPublication, rp *lksdk.RemoteParticipant) {
				c.onTrackPublished(pub, rp)
			},
			OnTrackUnpublished: func(pub *lksdk.RemoteTrackPublication, rp *lksdk.RemoteParticipant) {
				c.onTrackUnpublished(pub, rp)
			},
			OnTrackSubscribed: func(track *webrtc.TrackRemote, pub *lksdk.RemoteTrackPublication, rp *lksdk.RemoteParticipant) {
				c.onTrackSubscribed(track, pub, rp)
			},
			OnTrackUnsubscribed: func(track *webrtc.TrackRemote, pub *lksdk.RemoteTrackPublication, rp *lksdk.RemoteParticipant) {
				c.onTrackUnsubscribed(track, pub, rp)
			},
		},
	}

	rm, err := lksdk.ConnectToRoom(c.url, lksdk.ConnectInfo{
		APIKey:              c.apiKey,
		APISecret:           c.apiSecret,
		RoomName:            roomName,
		ParticipantIdentity: identity,
		ParticipantName:     "",
		ParticipantKind:     lksdk.ParticipantAgent,
	}, roomCB, lksdk.WithAutoSubscribe(false))

	if err != nil {
		return err
	}

	c.room = rm
	return nil
}

func (c *Client) Close() error {
	c.closed.Store(true)

	if c.room != nil {
		c.room.Disconnect()
		return nil
	}

	return errors.New("room is not connected")
}

// events pushing with backpresure / dropping policy
func (c *Client) push(evt Event) {
	if c.closed.Load() { return }

	select {
	case c.evCh <- evt:
		return
	default:
		switch c.dropPolicy  {
		case DropOldest:
			<-c.evCh
			c.evCh <- evt
		case Block:
			c.evCh <- evt
		}
	}
}

/// LiveKit events normalization and handling
func (c *Client) Events() <-chan Event {
	return c.evCh
}

func defaultListSubs(r *lksdk.Room) []*Subscription {
	subs := make([]*Subscription, 0, 16)
		for _, p := range r.GetRemoteParticipants() {
			for _, pub := range p.TrackPublications() {
					if pub.IsSubscribed() {
							subs = append(subs, &Subscription{
									TID: pub.SID(), PID: p.SID(),
									Kind: pub.Kind().String(), Source: pub.Source().String(),
							})
					}
			}
	}
	return subs
}

func (c *Client) SubscriptionSnapshot() []*Subscription {
	if c.room == nil { return nil }
	return c.listSubs(c.room)
}


// seams for participant and track publication
type remoteParticipant interface {
	SID() string
	Identity() string
}

type remoteTrackPublication interface {
	SID() string
	Kind() lksdk.TrackKind
}

type trackRemote interface {
	ID() string
	Kind() webrtc.RTPCodecType
}

// compile-time interface implementation assertions
var (
	_ remoteParticipant      = (*lksdk.RemoteParticipant)(nil)
	_ remoteTrackPublication = (*lksdk.RemoteTrackPublication)(nil)
)

func (c *Client) onParticipantJoined(p remoteParticipant) {
	evt := ParticipantJoined{ PID: p.SID(), Identity: p.Identity() }
	c.push(evt)
}

func (c *Client) onParticipantLeft(p remoteParticipant) {
	evt := ParticipantLeft{ PID: p.SID(), Identity: p.Identity() }
	c.push(evt)
}

func (c *Client) onTrackPublished(pub remoteTrackPublication, p remoteParticipant) {
	/// ToDo normalize track kind
	evt := TrackPublished{ PID: p.SID(), Identity: p.Identity(), TID: pub.SID(), Kind: pub.Kind().String()}
	c.push(evt)
}

func (c *Client) onTrackUnpublished(pub remoteTrackPublication, p remoteParticipant) {
	evt := TrackUnpublished{ PID: p.SID(), Identity: p.Identity(), TID: pub.SID() }
	c.push(evt)
}

func (c *Client) onRoomDisconnected(reason lksdk.DisconnectionReason) {
	c.push(RoomDisconnected{Reason: string(reason)})
}

func (c *Client) onRoomReconnected() {
	c.push(RoomReconnected{})
}

func (c *Client) onTrackSubscribed(track trackRemote, pub remoteTrackPublication, rp remoteParticipant) {
	c.push(TrackSubscribed{ PID: rp.SID(), Identity: rp.Identity(), TID: pub.SID(), Kind: pub.Kind().String(), Track: track })
}

func (c *Client) onTrackUnsubscribed(track trackRemote, pub remoteTrackPublication, rp remoteParticipant) {
	c.push(TrackUnsubscribed{ PID: rp.SID(), Identity: rp.Identity(), TID: pub.SID(), Kind: pub.Kind().String(), Track: track })
}

