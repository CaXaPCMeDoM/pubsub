package service

import (
	"context"
	"fmt"
	"pubsub/logger"
	"pubsub/protos/gen"
	"pubsub/subpub"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

type PubSubService struct {
	gen.UnimplementedPubSubServer
	subPub subpub.SubPub
	logger *logger.Logger
}

func NewPubSubService(subPub subpub.SubPub, logger *logger.Logger) *PubSubService {
	return &PubSubService{
		subPub: subPub,
		logger: logger,
	}
}

func (s *PubSubService) Subscribe(req *gen.SubscribeRequest, stream gen.PubSub_SubscribeServer) error {
	if req.Key == "" {
		return status.Error(codes.InvalidArgument, "key cannot be empty")
	}

	s.logger.Info("New subscription for key: %s", req.Key)

	msgCh := make(chan interface{}, 100)

	sub, err := s.subPub.Subscribe(req.Key, func(msg interface{}) {
		msgCh <- msg
	})
	if err != nil {
		s.logger.Error("Failed to subscribe: %v", err)
		return status.Error(codes.Internal, "failed to subscribe")
	}
	defer sub.Unsubscribe()

	for {
		select {
		case msg, ok := <-msgCh:
			if !ok {
				return status.Error(codes.Aborted, "subscription channel closed")
			}

			data, ok := msg.(string)
			if !ok {
				data = fmt.Sprintf("%v", msg)
			}

			if err := stream.Send(&gen.Event{Data: data}); err != nil {
				s.logger.Error("Failed to send message: %v", err)
				return status.Error(codes.Internal, "failed to send message")
			}
		case <-stream.Context().Done():
			s.logger.Info("Client disconnected from key: %s", req.Key)
			return nil
		}
	}
}

func (s *PubSubService) Publish(ctx context.Context, req *gen.PublishRequest) (*emptypb.Empty, error) {
	if req.Key == "" {
		return nil, status.Error(codes.InvalidArgument, "key cannot be empty")
	}

	s.logger.Info("Publishing message to key: %s", req.Key)

	if err := s.subPub.Publish(req.Key, req.Data); err != nil {
		s.logger.Error("Failed to publish: %v", err)
		return nil, status.Error(codes.Internal, "failed to publish message")
	}

	return &emptypb.Empty{}, nil
}
