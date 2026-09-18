package agent

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"strings"
	"time"

	"github.com/CodewithMubasher/NightCode/backend/internal/types"
)

// Run streams a canned echo response as RuntimeEvents via the provided channel.
func Run(ctx context.Context, userMessage string, events chan<- types.RuntimeEvent) {
	defer close(events)

	segmentID := fmt.Sprintf("seg-%d", time.Now().UnixNano())
	now := func() int64 { return time.Now().UnixMilli() }

	// turn.started
	events <- types.RuntimeEvent{
		Type:      "turn.started",
		ID:        fmt.Sprintf("evt-%d", time.Now().UnixNano()),
		SegmentID: segmentID,
		Timestamp: now(),
	}

	response := fmt.Sprintf(
		"You said: *%s*\n\nThis is a placeholder response from the **NightCode backend** — real agent logic comes in a later phase.\n\nFor now I'm just an echo. I'll repeat whatever you send me, formatted in markdown, so the streaming rendering path gets exercised.",
		userMessage,
	)

	words := strings.Fields(response)
	chunkSize := 3 + rand.Intn(3) // 3-5 words per chunk

	var accumulated strings.Builder
	for i := 0; i < len(words); i += chunkSize {
		end := i + chunkSize
		if end > len(words) {
			end = len(words)
		}
		chunk := strings.Join(words[i:end], " ") + " "
		accumulated.WriteString(chunk)

		select {
		case <-ctx.Done():
			log.Printf("echo agent: cancelled")
			events <- types.RuntimeEvent{
				Type:      "agent.error",
				ID:        fmt.Sprintf("evt-%d", time.Now().UnixNano()),
				SegmentID: segmentID,
				Timestamp: now(),
				Error:     "cancelled by user",
			}
			return
		case events <- types.RuntimeEvent{
			Type:      "assistant.delta",
			ID:        fmt.Sprintf("evt-%d", time.Now().UnixNano()),
			SegmentID: segmentID,
			Timestamp: now(),
			Text:      accumulated.String(),
		}:
		}

		time.Sleep(time.Duration(30+rand.Intn(30)) * time.Millisecond)
	}

	// DoneSentinel marks the end of the text stream
	select {
	case <-ctx.Done():
		events <- types.RuntimeEvent{
			Type:      "agent.error",
			ID:        fmt.Sprintf("evt-%d", time.Now().UnixNano()),
			SegmentID: segmentID,
			Timestamp: now(),
			Error:     "cancelled by user",
		}
		return
	case events <- types.RuntimeEvent{
		Type:      "assistant.delta",
		ID:        fmt.Sprintf("evt-%d", time.Now().UnixNano()),
		SegmentID: segmentID,
		Timestamp: now(),
		Text:      DoneSentinel,
	}:
	}

	// agent.completed
	events <- types.RuntimeEvent{
		Type:      "agent.completed",
		ID:        fmt.Sprintf("evt-%d", time.Now().UnixNano()),
		SegmentID: segmentID,
		Timestamp: now(),
	}
}
