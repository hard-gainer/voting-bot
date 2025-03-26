package notification

// MessageSender represents an interface for sending messages
type MessageSender interface {
	PostMessage(channelID, message string) error
}
