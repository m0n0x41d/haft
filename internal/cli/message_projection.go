package cli

import (
	"strings"

	"github.com/m0n0x41d/haft/internal/agent"
	"github.com/m0n0x41d/haft/internal/protocol"
)

const persistedFileAttachmentPrefix = "[File: "

func msgsToMsgInfos(messages []agent.Message) []protocol.MsgInfo {
	msgInfos := make([]protocol.MsgInfo, 0, len(messages))

	for _, message := range messages {
		msgInfos = append(msgInfos, msgInfoFromMessage(message))
	}

	return msgInfos
}

func msgInfoFromMessage(message agent.Message) protocol.MsgInfo {
	if message.Role != agent.RoleUser {
		return protocol.MsgInfo{
			ID:   message.ID,
			Role: string(message.Role),
			Text: message.Text(),
		}
	}

	return protocol.MsgInfo{
		ID:          message.ID,
		Role:        string(message.Role),
		Text:        projectedUserMessageText(message.Parts),
		Attachments: projectedUserMessageAttachments(message.Parts),
	}
}

func projectedUserMessageText(parts []agent.Part) string {
	var text strings.Builder

	for index, part := range parts {
		textPart, ok := part.(agent.TextPart)
		if !ok {
			continue
		}

		if index > 0 {
			if _, ok := parsePersistedFileAttachmentName(textPart.Text); ok {
				continue
			}
		}

		text.WriteString(textPart.Text)
	}

	return text.String()
}

func projectedUserMessageAttachments(parts []agent.Part) []protocol.MessageAttachment {
	attachments := make([]protocol.MessageAttachment, 0)

	for index, part := range parts {
		switch typedPart := part.(type) {
		case agent.ImagePart:
			attachments = append(attachments, protocol.MessageAttachment{
				Name:    typedPart.Filename,
				IsImage: true,
			})

		case agent.TextPart:
			if index == 0 {
				continue
			}

			name, ok := parsePersistedFileAttachmentName(typedPart.Text)
			if !ok {
				continue
			}

			attachments = append(attachments, protocol.MessageAttachment{
				Name:    name,
				IsImage: false,
			})
		}
	}

	if len(attachments) == 0 {
		return nil
	}

	return attachments
}

func persistedFileAttachmentText(name string, content string) string {
	return persistedFileAttachmentPrefix + name + "]\n" + content
}

func parsePersistedFileAttachmentName(text string) (string, bool) {
	if !strings.HasPrefix(text, persistedFileAttachmentPrefix) {
		return "", false
	}

	rest := strings.TrimPrefix(text, persistedFileAttachmentPrefix)
	name, _, found := strings.Cut(rest, "]\n")

	if !found || name == "" {
		return "", false
	}

	return name, true
}
