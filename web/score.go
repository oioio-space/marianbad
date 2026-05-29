//go:build js && wasm

package main

import (
	"encoding/json"

	"github.com/marianbad/internal/nim"
)

const scoreKey = "marianbad.score.v1"

type Score struct {
	VsAI      VsAIScore      `json:"vsAI"`
	TwoPlayer TwoPlayerScore `json:"twoPlayer"`
}

type VsAIScore struct {
	HumanWins int `json:"humanWins"`
	AIWins    int `json:"aiWins"`
}

type TwoPlayerScore struct {
	P1Wins int `json:"p1Wins"`
	P2Wins int `json:"p2Wins"`
}

func loadScore() Score {
	raw := localStorageGet(scoreKey)
	if raw == "" {
		return Score{}
	}
	var s Score
	if err := json.Unmarshal([]byte(raw), &s); err != nil {
		return Score{}
	}
	return s
}

func saveScore(s Score) {
	b, err := json.Marshal(s)
	if err != nil {
		return
	}
	localStorageSet(scoreKey, string(b))
}

func (s *Score) recordWin(mode nim.Mode, winner nim.Player) {
	switch mode {
	case nim.VsAI:
		if winner == nim.P1 {
			s.VsAI.HumanWins++
		} else {
			s.VsAI.AIWins++
		}
	case nim.TwoPlayer:
		if winner == nim.P1 {
			s.TwoPlayer.P1Wins++
		} else {
			s.TwoPlayer.P2Wins++
		}
	}
}

func (s *Score) resetMode(mode nim.Mode) {
	switch mode {
	case nim.VsAI:
		s.VsAI = VsAIScore{}
	case nim.TwoPlayer:
		s.TwoPlayer = TwoPlayerScore{}
	}
}
