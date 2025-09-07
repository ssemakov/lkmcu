package livekit

import (
	"errors"
	"sync"

	lksdk "github.com/livekit/server-sdk-go/v2"
	"github.com/pion/webrtc/v4"
)

type Room interface {
	Disconnect()
}

type Client struct {
	apiKey    string
	apiSecret string
	url       string
	room      Room

	evCh chan Event
	dropPolicy DropPolicy
	mu sync.RWMutex
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
	return &Client{url: url, apiKey: apiKey, apiSecret: apiSecret, evCh: make(chan Event, opts.BufferSize),
		dropPolicy: opts.DropPolicy }
}

func (c *Client) JoinRoom(roomName, identity string) error {
	roomCB := &lksdk.RoomCallback{
		OnParticipantConnected: c.onParticipantJoined,
		OnParticipantDisconnected: c.onParticipantLeft,
		OnDisconnected: func() { c.onRoomDisconnected(lksdk.OtherReason) },
		OnDisconnectedWithReason: c.onRoomDisconnected,
		OnReconnected: c.onRoomReconnected,

		ParticipantCallback: lksdk.ParticipantCallback{
			OnTrackPublished:  c.onTrackPublished,
			OnTrackUnpublished: c.onTrackUnpublished,
			OnTrackSubscribed: c.onTrackSubscribed,
			OnTrackUnsubscribed: c.onTrackUnsubscribed,
		},
	}

	rm, err := lksdk.ConnectToRoom(c.url, lksdk.ConnectInfo{
		APIKey:              c.apiKey,
		APISecret:           c.apiSecret,
		RoomName:            roomName,
		ParticipantIdentity: identity,
		ParticipantName:     "",
	}, roomCB, lksdk.WithAutoSubscribe(false))

	if err != nil {
		return err
	}

	c.room = rm
	return nil
}

func (c *Client) Close() error {
	close(c.evCh)

	if c.room != nil {
		c.room.Disconnect()
		return nil
	}
	return errors.New("room is not connected")
}

/// LiveKit events normalization and handling

func (c *Client) Events() <-chan Event {
	return c.evCh
}

func (c *Client) onParticipantJoined(p *lksdk.RemoteParticipant) {
	evt := ParticipantJoined{ PID: p.SID(), Identity: p.Identity() }
	c.push(evt)
}

func (c *Client) onParticipantLeft(p *lksdk.RemoteParticipant) {
	evt := ParticipantLeft{ PID: p.SID() }
	c.push(evt)
}

func (c *Client) onTrackPublished(pub *lksdk.RemoteTrackPublication, p *lksdk.RemoteParticipant) {
	/// ToDo normalize track kind
	evt := TrackPublished{ PID: p.SID(), TID: pub.SID(), Kind: pub.Kind().String() }
	c.push(evt)
}

func (c *Client) onTrackUnpublished(pub *lksdk.RemoteTrackPublication, p *lksdk.RemoteParticipant) {
	evt := TrackUnpublished{ PID: p.SID(), TID: pub.SID() }
	c.push(evt)
}

func (c *Client) onRoomDisconnected(reason lksdk.DisconnectionReason) {
	c.push(RoomDisconnected{Reason: string(reason)})
}

func (c *Client) onRoomReconnected() {
	c.push(RoomReconnected{})
}

func (c *Client) push(evt Event) {
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

func (c *Client) onTrackSubscribed(track *webrtc.TrackRemote, pub *lksdk.RemoteTrackPublication, rp *lksdk.RemoteParticipant) {
	c.push(TrackSubscribed{ PID: rp.SID(), TID: pub.SID(), Kind: pub.Kind().String(), Track: track })
}

func (c *Client) onTrackUnsubscribed(track *webrtc.TrackRemote, pub *lksdk.RemoteTrackPublication, rp *lksdk.RemoteParticipant) {
	c.push(TrackUnsubscribed{ PID: rp.SID(), TID: pub.SID(), Kind: pub.Kind().String(), Track: track })
}

