package livekit

import (
	"errors"

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
}

func NewClient(url, apiKey, apiSecret string) *Client {
	return &Client{url: url, apiKey: apiKey, apiSecret: apiSecret}
}

func (c *Client) JoinRoom(roomName, identity string) error {
	roomCB := &lksdk.RoomCallback{
		ParticipantCallback: lksdk.ParticipantCallback{
			OnTrackSubscribed: trackSubscribed,
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
	if c.room != nil {
		c.room.Disconnect()
		return nil
	}
	return errors.New("room is not connected")
}

func trackSubscribed(track *webrtc.TrackRemote, pub *lksdk.RemoteTrackPublication, rp *lksdk.RemoteParticipant) {
	// TODO: implement
}
