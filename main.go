package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
)

const (
	boardSize = 15
)

type MoveRequest struct {
	Board [][]int `json:"board"`
}

type MoveResponse struct {
	Board  [][]int `json:"board"`
	Winner int     `json:"winner,omitempty"`
	Error  string  `json:"error,omitempty"`
}

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "templates/index.html")
	})

	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	http.HandleFunc("/api/move", handleMove)

	fmt.Println("Server is running on http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func handleMove(w http.ResponseWriter, r *http.Request) {
	var req MoveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Check for winner after player's move
	if winner := checkWinner(req.Board); winner != 0 {
		json.NewEncoder(w).Encode(MoveResponse{Board: req.Board, Winner: winner})
		return
	}

	// Get AI move
	aiMove, err := getAIMove(req.Board)
	if err != nil {
		json.NewEncoder(w).Encode(MoveResponse{Error: err.Error(), Board: req.Board})
		return
	}

	if req.Board[aiMove.Row][aiMove.Col] == 0 {
		req.Board[aiMove.Row][aiMove.Col] = 2 // AI is player 2 (white)
	}

	// Check for winner after AI's move
	if winner := checkWinner(req.Board); winner != 0 {
		json.NewEncoder(w).Encode(MoveResponse{Board: req.Board, Winner: winner})
		return
	}

	json.NewEncoder(w).Encode(MoveResponse{Board: req.Board})
}

type AIMove struct {
	Row int `json:"row"`
	Col int `json:"col"`
}

func getAIMove(board [][]int) (*AIMove, error) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	apiURL := os.Getenv("OPENAI_API_URL")

	if apiKey == "" || apiURL == "" {
		return nil, fmt.Errorf("OPENAI_API_KEY and OPENAI_API_URL must be set")
	}

	prompt := "You are a Gomoku AI. The board is 15x15. 1 is black (player), 2 is white (you). It's your turn. Return your move as a JSON object with 'row' and 'col' keys. \n\nBoard:\n"
	for _, row := range board {
		prompt += fmt.Sprintf("%v\n", row)
	}
	prompt += "\nYour move:"

	reqBody := map[string]interface{}{
		"model": "minimax/minimax-m2:free",
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
		"response_format": map[string]string{"type": "json_object"},
	}

	reqBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(reqBytes))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var openAIResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := json.Unmarshal(respBody, &openAIResp); err != nil {
		return nil, fmt.Errorf("error unmarshalling openai response: %v, body: %s", err, string(respBody))
	}

	if len(openAIResp.Choices) == 0 {
		return nil, fmt.Errorf("no choices returned from AI")
	}

	var move AIMove
	if err := json.Unmarshal([]byte(openAIResp.Choices[0].Message.Content), &move); err != nil {
		return nil, fmt.Errorf("error unmarshalling move from AI: %v, content: %s", err, openAIResp.Choices[0].Message.Content)
	}

	return &move, nil
}

func checkWinner(board [][]int) int {
	// Check rows
	for r := 0; r < boardSize; r++ {
		for c := 0; c <= boardSize-5; c++ {
			if board[r][c] != 0 &&
				board[r][c] == board[r][c+1] &&
				board[r][c] == board[r][c+2] &&
				board[r][c] == board[r][c+3] &&
				board[r][c] == board[r][c+4] {
				return board[r][c]
			}
		}
	}

	// Check columns
	for c := 0; c < boardSize; c++ {
		for r := 0; r <= boardSize-5; r++ {
			if board[r][c] != 0 &&
				board[r][c] == board[r+1][c] &&
				board[r][c] == board[r+2][c] &&
				board[r][c] == board[r+3][c] &&
				board[r][c] == board[r+4][c] {
				return board[r][c]
			}
		}
	}

	// Check diagonals
	for r := 0; r <= boardSize-5; r++ {
		for c := 0; c <= boardSize-5; c++ {
			if board[r][c] != 0 &&
				board[r][c] == board[r+1][c+1] &&
				board[r][c] == board[r+2][c+2] &&
				board[r][c] == board[r+3][c+3] &&
				board[r][c] == board[r+4][c+4] {
				return board[r][c]
			}
		}
	}

	for r := 0; r <= boardSize-5; r++ {
		for c := 4; c < boardSize; c++ {
			if board[r][c] != 0 &&
				board[r][c] == board[r+1][c-1] &&
				board[r][c] == board[r+2][c-2] &&
				board[r][c] == board[r+3][c-3] &&
				board[r][c] == board[r+4][c-4] {
				return board[r][c]
			}
		}
	}

	return 0
}
