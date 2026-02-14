// Copyright (C) 2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package events

import "github.com/stretchr/testify/mock"

// MockPublisher is a mock implementation of Publisher interface
type MockPublisher struct {
	mock.Mock
}

func (m *MockPublisher) Publish(event Event) error {
	args := m.Called(event)
	return args.Error(0)
}

// MockSubscriber is a mock implementation of Subscriber interface
type MockSubscriber struct {
	mock.Mock
}

func (m *MockSubscriber) Subscribe(eventType EventType, handler func(Event)) error {
	args := m.Called(eventType, handler)
	return args.Error(0)
}

func (m *MockSubscriber) Unsubscribe(eventType EventType) error {
	args := m.Called(eventType)
	return args.Error(0)
}
