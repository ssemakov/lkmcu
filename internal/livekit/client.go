package livekit

import (
	"errors"
	"sync/atomic"
	"time"

	lksdk "github.com/livekit/server-sdk-go/v2"
	"github.com/pion/interceptor"
	"github.com/pion/rtp"
	"github.com/pion/webrtc/v4"
)

type setSubscribedFn func(subscribe bool) error
type findSubTargetFn func(pid, trackID string) (setSubscribedFn, error)
type listSubsFn func() []*Subscription

type Client struct {
	apiKey    string
	apiSecret string
	url       string
	room      *lksdk.Room

	evCh            chan Event
	dropPolicy      DropPolicy
	listSubsFn      listSubsFn
	findSubTargetFn findSubTargetFn
	closed          atomic.Bool
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
		opts = &EventOptions{BufferSize: 64, DropPolicy: DropOldest}
	}
	client := &Client{url: url, apiKey: apiKey, apiSecret: apiSecret,
		evCh:       make(chan Event, opts.BufferSize),
		dropPolicy: opts.DropPolicy,
	}
	client.findSubTargetFn = client.findSubTarget
	client.listSubsFn = client.defaultListSubsFn

	return client
}

func (c *Client) JoinRoom(roomName, identity string) error {
	roomCB := &lksdk.RoomCallback{
		OnParticipantConnected:    func(rp *lksdk.RemoteParticipant) { c.onParticipantJoined(rp) },
		OnParticipantDisconnected: func(rp *lksdk.RemoteParticipant) { c.onParticipantLeft(rp) },
		OnDisconnectedWithReason:  c.onRoomDisconnected,
		OnReconnected:             c.onRoomReconnected,

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

// Subscribe subscribes or unsubscribes to a track published by a participant.
// SDK API agnostic, relies on findSubTargetFn to map to the actual subscription function.
func (c *Client) Subscribe(pid string, trackID string, subscribe bool) error {
	if c.room == nil {
		return errors.New("room is not connected")
	}

	if c.findSubTargetFn == nil {
		panic("subscription target finder is not set")
	}

	subscribeFn, err := c.findSubTargetFn(pid, trackID)
	if err != nil {
		return err
	}

	return subscribeFn(subscribe)
}

// findSubTarget looks up the subscription function for a given participant ID and track ID.
// Returns an error if the participant or track is not found.
func (c *Client) findSubTarget(pid, trackID string) (setSubscribedFn, error) {
	rp := c.room.GetParticipantBySID(pid)
	if rp == nil {
		return nil, errors.New("participant not found: " + pid)
	}

	for _, pub := range rp.TrackPublications() {
		if pub.SID() == trackID {
			rpub, ok := pub.(*lksdk.RemoteTrackPublication)
			if !ok {
				return nil, errors.New("unexpected publication type for track: " + trackID)
			}
			return rpub.SetSubscribed, nil
		}
	}

	return nil, errors.New("track not found: " + trackID)
}

func (c *Client) Close() error {
	c.closed.Store(true)

	if c.room != nil {
		c.room.Disconnect()
		return nil
	}

	return errors.New("room is not connected")
}

// events pushing with back pressure / dropping policy
func (c *Client) push(evt Event) {
	if c.closed.Load() {
		return
	}

	select {
	case c.evCh <- evt:
		return
	default:
		switch c.dropPolicy {
		case DropOldest:
			<-c.evCh
			c.evCh <- evt
		case Block:
			c.evCh <- evt
		}
	}
}

// LiveKit events normalization and handling
func (c *Client) Events() <-chan Event {
	return c.evCh
}

func (c *Client) defaultListSubsFn() []*Subscription {
	subs := make([]*Subscription, 0, 16)
	room := c.room
	for _, p := range room.GetRemoteParticipants() {
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
	if c.room == nil {
		return nil
	}

	if c.listSubsFn == nil {
		panic("listSubsFn is not set")
	}

	return c.listSubsFn()
}

// seams for participant and track publication
type remoteParticipant interface {
	SID() string
	Identity() string
}

type remoteTrackPublication interface {
	SID() string
	Kind() lksdk.TrackKind
	Receiver() *webrtc.RTPReceiver
}

type webrtcRemoteTrack interface {
	ID() string
	SetReadDeadline(time.Time) error
	SSRC() webrtcSSRC
	ReadRTP() (*rtp.Packet, interceptor.Attributes, error)
}

type webrtcSSRC = webrtc.SSRC

// compile-time interface implementation assertions
var (
	_ remoteParticipant      = (*lksdk.RemoteParticipant)(nil)
	_ remoteTrackPublication = (*lksdk.RemoteTrackPublication)(nil)
	_ webrtcRemoteTrack      = (*webrtc.TrackRemote)(nil)
)

func (c *Client) onParticipantJoined(p remoteParticipant) {
	evt := ParticipantJoined{PID: p.SID(), Identity: p.Identity()}
	c.push(evt)
}

func (c *Client) onParticipantLeft(p remoteParticipant) {
	evt := ParticipantLeft{PID: p.SID(), Identity: p.Identity()}
	c.push(evt)
}

func (c *Client) onTrackPublished(pub remoteTrackPublication, p remoteParticipant) {
	/// ToDo normalize track kind
	evt := TrackPublished{PID: p.SID(), Identity: p.Identity(), TID: pub.SID(), Kind: pub.Kind().String()}
	c.push(evt)
}

func (c *Client) onTrackUnpublished(pub remoteTrackPublication, p remoteParticipant) {
	evt := TrackUnpublished{PID: p.SID(), Identity: p.Identity(), TID: pub.SID()}
	c.push(evt)
}

func (c *Client) onRoomDisconnected(reason lksdk.DisconnectionReason) {
	if reason == lksdk.RoomClosed {
		c.push(RoomClosed{})
		return
	}
	c.push(RoomDisconnected{Reason: string(reason)})
}

func (c *Client) onRoomReconnected() {
	c.push(RoomReconnected{})
}

func (c *Client) onTrackSubscribed(track webrtcRemoteTrack, pub remoteTrackPublication, rp remoteParticipant) {
	rt := &remoteTrackAdapter{track: track}

	r := pub.Receiver()
	rec := &receiverAdapter{receiver: r}

	c.push(TrackSubscribed{PID: rp.SID(), Identity: rp.Identity(), TID: pub.SID(), Kind: pub.Kind().String(),
		RemoteTrack: rt, Receiver: rec})
}

func (c *Client) onTrackUnsubscribed(track webrtcRemoteTrack, pub remoteTrackPublication, rp remoteParticipant) {
	rt := &remoteTrackAdapter{track: track}
	c.push(TrackUnsubscribed{PID: rp.SID(), Identity: rp.Identity(), TID: pub.SID(), Kind: pub.Kind().String(),
		RemoteTrack: rt})
}
