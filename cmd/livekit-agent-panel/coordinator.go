package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"
)

// Coordinator orchestrates turn-taking between panelists.
type Coordinator struct {
	Panelists     []*Panelist
	Transcript    *Transcript
	Topic         string
	CurrentRound  int
	speakerPause  time.Duration
	introComplete bool
	llmClient     LLMClient
}

// NewCoordinator creates a new coordinator.
func NewCoordinator(panelists []*Panelist, transcript *Transcript, topic string, llmClient LLMClient) *Coordinator {
	return &Coordinator{
		Panelists:    panelists,
		Transcript:   transcript,
		Topic:        topic,
		CurrentRound: 0,
		speakerPause: 1500 * time.Millisecond,
		llmClient:    llmClient,
	}
}

// SetSpeakerPause sets the pause duration between speakers.
func (c *Coordinator) SetSpeakerPause(d time.Duration) {
	c.speakerPause = d
}

// RunIntroductions has each panelist introduce themselves briefly.
func (c *Coordinator) RunIntroductions(ctx context.Context) error {
	if c.introComplete {
		return nil
	}

	fmt.Println("[Coordinator] Running panelist introductions...")

	for i, panelist := range c.Panelists {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		response, err := panelist.GenerateResponse(ctx, c.Transcript, "Please introduce yourself briefly to the panel in 1 sentence.")
		if err != nil {
			log.Printf("[%s] Error generating introduction: %v", panelist.Name, err)
			continue
		}

		fmt.Printf("[%s] %s\n", panelist.Name, response)

		if err := panelist.Speak(ctx, response); err != nil {
			log.Printf("[%s] Error speaking introduction: %v", panelist.Name, err)
		}

		c.Transcript.Add(panelist.Name, response)

		if i < len(c.Panelists)-1 {
			time.Sleep(c.speakerPause)
		}
	}

	c.introComplete = true
	fmt.Println("[Coordinator] Introductions complete. Waiting for moderator questions...")
	return nil
}

// OnModeratorSpeech handles when the moderator speaks.
func (c *Coordinator) OnModeratorSpeech(ctx context.Context, text string) error {
	fmt.Printf("[Moderator] %s\n", text)
	c.Transcript.Add("Moderator", text)

	return c.RunDiscussionRound(ctx, text)
}

// RunDiscussionRound has all panelists respond to the moderator's question.
func (c *Coordinator) RunDiscussionRound(ctx context.Context, question string) error {
	c.CurrentRound++
	fmt.Printf("\n[Coordinator] Starting round %d\n", c.CurrentRound)

	// Get speaking order based on relevance to the question
	order := c.SelectSpeakingOrder(ctx, question)
	fmt.Printf("[Coordinator] Speaking order: %s\n", FormatNames(order))

	for i, panelist := range order {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		fmt.Printf("[%s] Thinking...\n", panelist.Name)

		response, err := panelist.GenerateResponse(ctx, c.Transcript, question)
		if err != nil {
			log.Printf("[%s] Error generating response: %v", panelist.Name, err)
			continue
		}

		fmt.Printf("[%s] %s\n", panelist.Name, response)

		if err := panelist.Speak(ctx, response); err != nil {
			log.Printf("[%s] Error speaking: %v", panelist.Name, err)
		}

		c.Transcript.Add(panelist.Name, response)

		if i < len(order)-1 {
			time.Sleep(c.speakerPause)
		}
	}

	fmt.Printf("[Coordinator] Round %d complete. Waiting for next question...\n\n", c.CurrentRound)
	return nil
}

// SelectSpeakingOrder returns panelists ordered by relevance to the question.
func (c *Coordinator) SelectSpeakingOrder(ctx context.Context, question string) []*Panelist {
	n := len(c.Panelists)
	if n == 0 {
		return nil
	}

	// Try to get relevance-based order
	order, err := c.rankByRelevance(ctx, question)
	if err != nil {
		log.Printf("[Coordinator] Relevance ranking failed, using rotation: %v", err)
		return c.FallbackOrder()
	}

	return order
}

// rankByRelevance uses an LLM to rank panelists by relevance to the question.
func (c *Coordinator) rankByRelevance(ctx context.Context, question string) ([]*Panelist, error) {
	prompt := BuildRankingPrompt(question, c.Panelists)

	text, err := c.llmClient.Generate(ctx, "", prompt, 100)
	if err != nil {
		return nil, err
	}

	names, err := ParseRankingResponse(text)
	if err != nil {
		return nil, err
	}

	return MapNamesToOrder(names, c.Panelists), nil
}

// BuildRankingPrompt builds the prompt for ranking panelists by relevance.
// Exported for testing.
func BuildRankingPrompt(question string, panelists []*Panelist) string {
	var descriptions []string
	for _, p := range panelists {
		descriptions = append(descriptions, fmt.Sprintf("- %s: %s", p.Name, p.Personality))
	}

	return fmt.Sprintf(`Given this question from a panel moderator:
"%s"

And these panelists with their backgrounds:
%s

Rank the panelists from MOST to LEAST relevant to answer this specific question.
Consider whose expertise, perspective, or background makes them best suited to speak first.

Return ONLY a JSON array of names in order, like: ["Name1", "Name2", "Name3"]
No explanation, just the JSON array.`, question, strings.Join(descriptions, "\n"))
}

// ParseRankingResponse parses the LLM response into a list of names.
// Exported for testing.
func ParseRankingResponse(response string) ([]string, error) {
	text := strings.TrimSpace(response)
	var names []string
	if err := json.Unmarshal([]byte(text), &names); err != nil {
		return nil, fmt.Errorf("parse names: %w (got: %s)", err, text)
	}
	return names, nil
}

// MapNamesToOrder maps a list of names to panelists in order.
// Any panelists not in the list are appended at the end.
// Exported for testing.
func MapNamesToOrder(names []string, panelists []*Panelist) []*Panelist {
	nameToPanel := make(map[string]*Panelist)
	for _, p := range panelists {
		nameToPanel[p.Name] = p
	}

	var order []*Panelist
	seen := make(map[string]bool)
	for _, name := range names {
		if p, ok := nameToPanel[name]; ok && !seen[name] {
			order = append(order, p)
			seen[name] = true
		}
	}

	// Add any missing panelists at the end
	for _, p := range panelists {
		if !seen[p.Name] {
			order = append(order, p)
		}
	}

	return order
}

// FallbackOrder returns a simple rotation-based order.
// Exported for testing.
func (c *Coordinator) FallbackOrder() []*Panelist {
	n := len(c.Panelists)
	if n == 0 {
		return nil
	}
	startIdx := (c.CurrentRound - 1) % n
	order := make([]*Panelist, n)
	for i := 0; i < n; i++ {
		order[i] = c.Panelists[(startIdx+i)%n]
	}
	return order
}

// PanelistNames returns the names of all panelists.
func (c *Coordinator) PanelistNames() []string {
	names := make([]string, len(c.Panelists))
	for i, p := range c.Panelists {
		names[i] = p.Name
	}
	return names
}

// FormatNames returns a formatted list of panelist names.
// Exported for testing.
func FormatNames(panelists []*Panelist) string {
	names := make([]string, len(panelists))
	for i, p := range panelists {
		names[i] = p.Name
	}
	return strings.Join(names, " -> ")
}

// RunAutoPanel runs a fully automated panel discussion with an AI moderator.
func (c *Coordinator) RunAutoPanel(ctx context.Context, moderator *Moderator) error {
	fmt.Println()
	fmt.Println("===========================================")
	fmt.Println("  Starting Automated Panel Discussion")
	fmt.Println("===========================================")
	fmt.Println()

	// 1. Moderator introduction
	fmt.Printf("[%s] Generating introduction...\n", moderator.Name)
	intro, err := moderator.GenerateIntroduction(ctx, c.Topic, c.PanelistNames())
	if err != nil {
		return fmt.Errorf("generate introduction: %w", err)
	}
	fmt.Printf("[%s] %s\n", moderator.Name, intro)

	if err := moderator.Speak(ctx, intro); err != nil {
		log.Printf("[%s] Error speaking introduction: %v", moderator.Name, err)
	}
	c.Transcript.Add(moderator.Name, intro)
	time.Sleep(c.speakerPause)

	// 2. Panelist introductions
	if err := c.RunIntroductions(ctx); err != nil {
		return fmt.Errorf("panelist introductions: %w", err)
	}
	time.Sleep(c.speakerPause)

	// 3. Discussion rounds
	totalRounds := moderator.TotalRounds()
	for round := 1; round <= totalRounds; round++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		fmt.Printf("\n[%s] Generating question for round %d/%d...\n", moderator.Name, round, totalRounds)

		question, err := moderator.GenerateQuestion(ctx, c.Transcript, round)
		if err != nil {
			log.Printf("[%s] Error generating question: %v", moderator.Name, err)
			continue
		}
		fmt.Printf("[%s] %s\n", moderator.Name, question)

		if err := moderator.Speak(ctx, question); err != nil {
			log.Printf("[%s] Error speaking question: %v", moderator.Name, err)
		}
		c.Transcript.Add(moderator.Name, question)
		time.Sleep(c.speakerPause)

		// Run the discussion round
		if err := c.RunDiscussionRound(ctx, question); err != nil {
			return fmt.Errorf("discussion round %d: %w", round, err)
		}

		time.Sleep(c.speakerPause)
	}

	// 4. Closing statement
	fmt.Printf("\n[%s] Generating closing statement...\n", moderator.Name)
	closing, err := moderator.GenerateClosing(ctx, c.Transcript)
	if err != nil {
		log.Printf("[%s] Error generating closing: %v", moderator.Name, err)
	} else {
		fmt.Printf("[%s] %s\n", moderator.Name, closing)
		if err := moderator.Speak(ctx, closing); err != nil {
			log.Printf("[%s] Error speaking closing: %v", moderator.Name, err)
		}
		c.Transcript.Add(moderator.Name, closing)
	}

	fmt.Println()
	fmt.Println("===========================================")
	fmt.Println("  Panel Discussion Complete")
	fmt.Println("===========================================")
	fmt.Println()

	return nil
}
