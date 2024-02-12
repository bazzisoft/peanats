package stream

import (
	"context"
	"io"
	"strconv"
	"testing"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestReceiverImpl_Receive(t *testing.T) {
	t.Run("ok", func(t *testing.T) {
		uid := nuid.Next()
		sub := subscriptionMock{}
		var rcv Receiver = &receiverImpl{
			sub: &sub,
			uid: uid,
			seq: 0,
		}
		for i := 0; i < 3; i++ {
			sub.On("NextMsg", mock.Anything).Return(&nats.Msg{
				Header: nats.Header{
					HeaderUID:      []string{uid},
					HeaderSequence: []string{strconv.Itoa(i)},
				},
				Data: []byte("the parson had a dog"),
			}, nil).Once()
			msg, err := rcv.Receive(context.Background())
			require.NoError(t, err)
			require.Equal(t, uid, msg.Header.Get(HeaderUID))
		}
		sub.On("NextMsg", mock.Anything).Return(nil, io.EOF).Once()
		msg, err := rcv.Receive(context.Background())
		require.Nil(t, msg)
		require.ErrorIs(t, err, io.EOF)

		sub.AssertExpectations(t)
	})
	t.Run("sequence header absent", func(t *testing.T) {
		uid := nuid.Next()
		sub := subscriptionMock{}
		var rcv Receiver = &receiverImpl{
			sub: &sub,
			uid: uid,
			seq: 0,
		}
		sub.On("NextMsg", mock.Anything).Return(&nats.Msg{
			Header: nats.Header{
				HeaderUID: []string{uid},
			},
		}, nil).Once()
		msg, err := rcv.Receive(context.Background())
		require.Nil(t, msg)
		require.ErrorIs(t, err, ErrProtocolViolation)
		sub.AssertExpectations(t)
	})
	t.Run("sequence out of order", func(t *testing.T) {
		uid := nuid.Next()
		sub := subscriptionMock{}
		var rcv Receiver = &receiverImpl{
			sub: &sub,
			uid: uid,
			seq: 0,
		}
		sub.On("NextMsg", mock.Anything).Return(&nats.Msg{
			Header: nats.Header{
				HeaderUID:      []string{uid},
				HeaderSequence: []string{strconv.Itoa(100500)},
			},
		}, nil).Once()
		msg, err := rcv.Receive(context.Background())
		require.Nil(t, msg)
		require.ErrorIs(t, err, ErrProtocolViolation)
		sub.AssertExpectations(t)
	})
	t.Run("sequence malformed", func(t *testing.T) {
		uid := nuid.Next()
		sub := subscriptionMock{}
		var rcv Receiver = &receiverImpl{
			sub: &sub,
			uid: uid,
			seq: 0,
		}
		sub.On("NextMsg", mock.Anything).Return(&nats.Msg{
			Header: nats.Header{
				HeaderUID:      []string{uid},
				HeaderSequence: []string{"not a number"},
			},
		}, nil).Once()
		msg, err := rcv.Receive(context.Background())
		require.Nil(t, msg)
		require.ErrorIs(t, err, ErrProtocolViolation)
		sub.AssertExpectations(t)
	})
	t.Run("uid absent", func(t *testing.T) {
		uid := nuid.Next()
		sub := subscriptionMock{}
		var rcv Receiver = &receiverImpl{
			sub: &sub,
			uid: uid,
			seq: 0,
		}
		sub.On("NextMsg", mock.Anything).Return(&nats.Msg{
			Header: nats.Header{
				HeaderSequence: []string{strconv.Itoa(0)},
			},
		}, nil).Once()
		msg, err := rcv.Receive(context.Background())
		require.Nil(t, msg)
		require.ErrorIs(t, err, ErrProtocolViolation)
		sub.AssertExpectations(t)
	})
	t.Run("done message with data", func(t *testing.T) {
		uid := nuid.Next()
		sub := subscriptionMock{}
		var rcv Receiver = &receiverImpl{
			sub: &sub,
			uid: uid,
			seq: 0,
		}
		sub.On("NextMsg", mock.Anything).Return(&nats.Msg{
			Header: nats.Header{
				HeaderUID:      []string{uid},
				HeaderSequence: []string{strconv.Itoa(0)},
				HeaderControl:  []string{HeaderControlDone},
			},
			Data: []byte("the parson had a dog"),
		}, nil).Once()
		msg, err := rcv.Receive(context.Background())
		require.Nil(t, msg)
		require.ErrorIs(t, err, ErrProtocolViolation)
		sub.AssertExpectations(t)
	})
}

func TestReceiverImpl_ReceiveAll(t *testing.T) {
	uid := nuid.Next()
	sub := subscriptionMock{}
	var rcv Receiver = &receiverImpl{
		sub: &sub,
		uid: uid,
		seq: 0,
	}
	for i := 0; i < 10; i++ {
		sub.On("NextMsg", mock.Anything).Return(&nats.Msg{
			Header: nats.Header{
				HeaderUID:      []string{uid},
				HeaderSequence: []string{strconv.Itoa(i)},
			},
			Data: []byte("the parson had a dog"),
		}, nil).Once()
	}
	sub.On("NextMsg", mock.Anything).Return(nil, io.EOF).Once()

	stream, err := rcv.ReceiveAll(context.Background())
	require.NoError(t, err)
	require.Len(t, stream, 10)
	for i, msg := range stream {
		require.Equal(t, uid, msg.Header.Get(HeaderUID))
		require.Equal(t, strconv.Itoa(i), msg.Header.Get(HeaderSequence))
		require.Equal(t, "the parson had a dog", string(msg.Data))
	}

	sub.AssertExpectations(t)
}

type subscriptionMock struct {
	mock.Mock
}

func (m *subscriptionMock) Unsubscribe() error {
	args := m.Called()
	return args.Error(0)
}

func (m *subscriptionMock) NextMsg(ctx context.Context) (*nats.Msg, error) {
	args := m.Called(ctx)
	if err := args.Error(1); err != nil {
		return nil, err
	}
	return args.Get(0).(*nats.Msg), nil
}
