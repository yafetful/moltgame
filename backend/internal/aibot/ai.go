package aibot

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"time"
)

const (
	openRouterURL = "https://openrouter.ai/api/v1/chat/completions"
	aiTimeout     = 30 * time.Second
)

type aiDecision struct {
	Action map[string]interface{}
	Reason string
}

const systemPrompt = `You are an aggressive, skilled Texas Hold'em poker player in a 6-player tournament. You play to WIN, not just survive.

ABSOLUTE RULES:
- "action" MUST be exactly one of the actions listed under "Valid actions" — NEVER choose an action not listed there
- "amount" is for "raise" only (between min and max). For other actions set amount to 0
- "reason" max 10 words
- NEVER fold when you can check — checking is FREE

STRATEGY:
- Strong hands (top pair+, overpair, two pair, sets, straights, flushes): RAISE aggressively to build the pot. Do NOT just check or call with strong hands.
- Draws (flush draw, straight draw, open-ender): semi-bluff with a RAISE, especially in position.
- BLUFF 15-25% of the time when: you are BTN/late position, the board is scary, or opponents showed weakness.
- Pot odds: if call cost < 30% of pot and you have any draw or pair, CALL.
- Short-stacked (<15 big blinds): push all-in with any pair, Ace-high, or K-Q+.
- Big blind defense: don't fold easily — you already invested chips.
- Preflop: raise 2.5-3x BB with playable hands (pairs, suited connectors, broadway cards). Don't limp.
- Postflop bet sizing: 50-75% of pot for value bets, 33-50% for bluffs.
- Vary your play — don't always take the same action with the same hand type.`

var pokerActionSchema = map[string]interface{}{
	"type": "json_schema",
	"json_schema": map[string]interface{}{
		"name":   "poker_action",
		"strict": true,
		"schema": map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"action": map[string]interface{}{
					"type":        "string",
					"description": "MUST be one of the actions listed under 'Valid actions' in the prompt. Do NOT choose an action not listed there.",
				},
				"amount": map[string]interface{}{
					"type":        "number",
					"description": "Bet amount, only required for raise action. Must be between min and max allowed.",
				},
				"reason": map[string]interface{}{
					"type":        "string",
					"description": "Very short reason, max 10 words.",
				},
			},
			"required":             []string{"action", "amount", "reason"},
			"additionalProperties": false,
		},
	},
}

func callAI(orKey, model, agentName string, state map[string]interface{}, nameByID map[string]string) aiDecision {
	userPrompt := formatStatePrompt(agentName, state, nameByID)

	reqBody, _ := json.Marshal(map[string]interface{}{
		"model": model,
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": userPrompt},
		},
		"temperature":     0.7,
		"max_tokens":      500,
		"response_format": pokerActionSchema,
	})

	req, _ := http.NewRequest("POST", openRouterURL, bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+orKey)

	client := &http.Client{Timeout: aiTimeout}
	resp, err := client.Do(req)
	if err != nil {
		slog.Warn("OpenRouter request failed", "agent", agentName, "error", err)
		return fallbackDecision(state)
	}
	defer resp.Body.Close()

	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		slog.Warn("OpenRouter error", "agent", agentName, "status", resp.StatusCode, "body", string(data[:min(len(data), 200)]))
		return fallbackDecision(state)
	}

	var orResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(data, &orResp); err != nil || len(orResp.Choices) == 0 {
		slog.Warn("failed to parse OpenRouter response", "agent", agentName)
		return fallbackDecision(state)
	}

	content := strings.TrimSpace(orResp.Choices[0].Message.Content)
	if content == "" {
		slog.Warn("OpenRouter returned empty content", "agent", agentName)
		return fallbackDecision(state)
	}
	return parseAIResponse(content, state, agentName)
}

func parseAIResponse(content string, state map[string]interface{}, agentName string) aiDecision {
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(content), &parsed); err != nil {
		re := regexp.MustCompile("(?s)```(?:json)?\\s*(.+?)\\s*```")
		if matches := re.FindStringSubmatch(content); len(matches) > 1 {
			if err2 := json.Unmarshal([]byte(matches[1]), &parsed); err2 != nil {
				slog.Warn("AI response not valid JSON", "agent", agentName, "content", content[:min(len(content), 100)])
				return fallbackDecision(state)
			}
		} else {
			slog.Warn("AI response not valid JSON", "agent", agentName, "content", content[:min(len(content), 100)])
			return fallbackDecision(state)
		}
	}

	actionType, _ := parsed["action"].(string)
	reason, _ := parsed["reason"].(string)

	if actionType == "" {
		slog.Warn("AI returned no action type", "agent", agentName)
		return fallbackDecision(state)
	}

	validActions := extractValidActions(state)
	valid := false
	for _, va := range validActions {
		if va["type"] == actionType {
			valid = true
			break
		}
	}
	if !valid {
		slog.Warn("AI chose invalid action, falling back", "agent", agentName, "action", actionType)
		return fallbackDecision(state)
	}

	action := map[string]interface{}{"type": actionType}
	if actionType == "raise" {
		if amt, ok := parsed["amount"].(float64); ok {
			for _, va := range validActions {
				if va["type"] == "raise" {
					minAmt, _ := va["min_amount"].(float64)
					maxAmt, _ := va["max_amount"].(float64)
					if amt < minAmt {
						amt = minAmt
					}
					if amt > maxAmt {
						amt = maxAmt
					}
					break
				}
			}
			action["amount"] = int(amt)
		} else {
			for _, va := range validActions {
				if va["type"] == "raise" {
					action["amount"] = int(va["min_amount"].(float64))
					break
				}
			}
		}
	}

	return aiDecision{Action: action, Reason: reason}
}

func fallbackDecision(state map[string]interface{}) aiDecision {
	actions := extractValidActions(state)
	fb := buildFallback(actions)
	return aiDecision{Action: fb, Reason: "(AI fallback)"}
}

func buildFallback(actions []map[string]interface{}) map[string]interface{} {
	for _, act := range actions {
		if act["type"] == "check" {
			return map[string]interface{}{"type": "check"}
		}
	}
	for _, act := range actions {
		if act["type"] == "call" {
			return map[string]interface{}{"type": "call"}
		}
	}
	return map[string]interface{}{"type": "fold"}
}

func extractValidActions(state map[string]interface{}) []map[string]interface{} {
	rawActions, ok := state["valid_actions"]
	if !ok || rawActions == nil {
		return nil
	}
	arr, ok := rawActions.([]interface{})
	if !ok {
		return nil
	}
	result := make([]map[string]interface{}, 0, len(arr))
	for _, raw := range arr {
		if m, ok := raw.(map[string]interface{}); ok {
			result = append(result, m)
		}
	}
	return result
}

// --- Prompt formatting ---

func formatStatePrompt(agentID string, state map[string]interface{}, nameByID map[string]string) string {
	var sb strings.Builder

	phase, _ := state["phase"].(string)
	handNum := 0
	if h, ok := state["hand_num"].(float64); ok {
		handNum = int(h)
	}
	sb.WriteString(fmt.Sprintf("Hand #%d, Phase: %s\n", handNum, phase))

	// Determine dealer, SB, BB seats for position labels
	dealerSeat := -1
	if ds, ok := state["dealer_seat"].(float64); ok {
		dealerSeat = int(ds)
	}
	var activeSeats []int
	if players, ok := state["players"].([]interface{}); ok {
		for _, raw := range players {
			p := raw.(map[string]interface{})
			eliminated, _ := p["eliminated"].(bool)
			if !eliminated {
				seat := 0
				if s, ok := p["seat"].(float64); ok {
					seat = int(s)
				}
				activeSeats = append(activeSeats, seat)
			}
		}
	}
	sbSeat, bbSeat := -1, -1
	if dealerSeat >= 0 && len(activeSeats) >= 2 {
		di := -1
		for i, s := range activeSeats {
			if s == dealerSeat {
				di = i
				break
			}
		}
		if di >= 0 {
			if len(activeSeats) == 2 {
				sbSeat = activeSeats[di]
				bbSeat = activeSeats[(di+1)%len(activeSeats)]
			} else {
				sbSeat = activeSeats[(di+1)%len(activeSeats)]
				bbSeat = activeSeats[(di+2)%len(activeSeats)]
			}
		}
	}

	// Hole cards + your position
	myChips := 0
	myTotalBet := 0
	if players, ok := state["players"].([]interface{}); ok {
		for _, raw := range players {
			p := raw.(map[string]interface{})
			if p["id"] == agentID {
				if hole, ok := p["hole"].([]interface{}); ok && len(hole) > 0 {
					cards := make([]string, len(hole))
					for i, c := range hole {
						cards[i] = fmt.Sprintf("%v", c)
					}
					sb.WriteString(fmt.Sprintf("Your cards: %s\n", strings.Join(cards, ", ")))
				}
				if c, ok := p["chips"].(float64); ok {
					myChips = int(c)
				}
				if tb, ok := p["total_bet"].(float64); ok {
					myTotalBet = int(tb)
				}
				mySeat := 0
				if s, ok := p["seat"].(float64); ok {
					mySeat = int(s)
				}
				pos := positionLabel(mySeat, dealerSeat, sbSeat, bbSeat)
				sb.WriteString(fmt.Sprintf("Your position: %s\n", pos))
				break
			}
		}
	}

	// Community cards
	if community, ok := state["community"].([]interface{}); ok && len(community) > 0 {
		cards := make([]string, len(community))
		for i, c := range community {
			cards[i] = fmt.Sprintf("%v", c)
		}
		sb.WriteString(fmt.Sprintf("Community: %s\n", strings.Join(cards, ", ")))
	}

	// Pot
	totalPot := 0
	if pots, ok := state["pots"].([]interface{}); ok {
		for _, raw := range pots {
			pot := raw.(map[string]interface{})
			if amt, ok := pot["amount"].(float64); ok {
				totalPot += int(amt)
			}
		}
		sb.WriteString(fmt.Sprintf("Pot: %d\n", totalPot))
	}

	// Blinds
	if smallBlind, ok := state["small_blind"].(float64); ok {
		bigBlind, _ := state["big_blind"].(float64)
		sb.WriteString(fmt.Sprintf("Blinds: %d/%d\n", int(smallBlind), int(bigBlind)))
	}

	// Current bet to match
	if cb, ok := state["current_bet"].(float64); ok && int(cb) > 0 {
		sb.WriteString(fmt.Sprintf("Current bet to match: %d\n", int(cb)))
	}

	// Your investment info
	if myTotalBet > 0 {
		sb.WriteString(fmt.Sprintf("You already invested: %d this hand\n", myTotalBet))
	}
	if myChips > 0 && totalPot > 0 {
		spr := float64(myChips) / float64(totalPot)
		sb.WriteString(fmt.Sprintf("Your stack/pot ratio: %.1f\n", spr))
	}

	// Players
	sb.WriteString("\nPlayers:\n")
	if players, ok := state["players"].([]interface{}); ok {
		for _, raw := range players {
			p := raw.(map[string]interface{})
			id, _ := p["id"].(string)
			name := nameByID[id]
			if name == "" && len(id) >= 8 {
				name = id[:8]
			}
			chips := 0
			if c, ok := p["chips"].(float64); ok {
				chips = int(c)
			}
			bet := 0
			if b, ok := p["bet"].(float64); ok {
				bet = int(b)
			}
			seat := 0
			if s, ok := p["seat"].(float64); ok {
				seat = int(s)
			}
			folded, _ := p["folded"].(bool)
			allIn, _ := p["all_in"].(bool)
			eliminated, _ := p["eliminated"].(bool)
			disconnected, _ := p["disconnected"].(bool)

			marker := ""
			if id == agentID {
				marker = " (YOU)"
			}
			status := ""
			if eliminated {
				status = " [eliminated]"
			} else if disconnected {
				status = " [disconnected]"
			} else if folded {
				status = " [folded]"
			} else if allIn {
				status = " [all-in]"
			}

			pos := positionLabel(seat, dealerSeat, sbSeat, bbSeat)
			sb.WriteString(fmt.Sprintf("- %s%s [%s]: %d chips, bet %d%s\n", name, marker, pos, chips, bet, status))
		}
	}

	// Valid actions
	sb.WriteString("\nValid actions:\n")
	if actions, ok := state["valid_actions"].([]interface{}); ok {
		for _, raw := range actions {
			act := raw.(map[string]interface{})
			actType, _ := act["type"].(string)
			switch actType {
			case "fold":
				sb.WriteString("- fold\n")
			case "check":
				sb.WriteString("- check (FREE - costs nothing)\n")
			case "call":
				cost := 0
				if c, ok := act["call_cost"].(float64); ok {
					cost = int(c)
				}
				sb.WriteString(fmt.Sprintf("- call (cost: %d)\n", cost))
			case "raise":
				minAmt, maxAmt := 0, 0
				if m, ok := act["min_amount"].(float64); ok {
					minAmt = int(m)
				}
				if m, ok := act["max_amount"].(float64); ok {
					maxAmt = int(m)
				}
				sb.WriteString(fmt.Sprintf("- raise (min: %d, max: %d)\n", minAmt, maxAmt))
			case "allin":
				amt := 0
				if m, ok := act["min_amount"].(float64); ok {
					amt = int(m)
				}
				sb.WriteString(fmt.Sprintf("- allin (amount: %d)\n", amt))
			}
		}
	}

	return sb.String()
}

func positionLabel(seat, dealerSeat, sbSeat, bbSeat int) string {
	if seat == dealerSeat {
		return "BTN"
	}
	if seat == sbSeat {
		return "SB"
	}
	if seat == bbSeat {
		return "BB"
	}
	return "MP"
}
