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
		log.Printf("AI move error: %v", err)
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

	log.Printf("Using API URL: %s", apiURL)

	prompt := "You are a Gomoku AI. The board is 15x15. 1 is black (player), 2 is white (you). Empty cells are 0. It's your turn.\n\n"
	prompt += "IMPORTANT: Reply with ONLY a JSON object in this exact format: {\"row\": number, \"col\": number}\n"
	prompt += "Do NOT include any explanation or text before or after the JSON.\n\n"
	prompt += "Board state:\n"
	for i, row := range board {
		prompt += fmt.Sprintf("Row %2d: %v\n", i, row)
	}
	prompt += "\nYour move (JSON only):"

	reqBody := map[string]interface{}{
		"model": "microsoft/phi-4-reasoning",
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
	}

	reqBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("error marshalling request: %v", err)
	}

	log.Printf("Request body: %s", string(reqBytes))

	req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(reqBytes))
	if err != nil {
		return nil, fmt.Errorf("error creating request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error making request: %v", err)
	}
	defer resp.Body.Close()

	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response: %v", err)
	}

	log.Printf("Response status: %d", resp.StatusCode)
	log.Printf("Response body: %s", string(respBody))

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(respBody))
	}

	// Try to parse as generic JSON first to see the structure
	var genericResp map[string]interface{}
	if err := json.Unmarshal(respBody, &genericResp); err != nil {
		return nil, fmt.Errorf("error unmarshalling response as generic JSON: %v", err)
	}

	log.Printf("Parsed response structure: %+v", genericResp)

	var openAIResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := json.Unmarshal(respBody, &openAIResp); err != nil {
		return nil, fmt.Errorf("error unmarshalling openai response: %v", err)
	}

	if len(openAIResp.Choices) == 0 {
		return nil, fmt.Errorf("no choices returned from AI. Full response: %s", string(respBody))
	}

	log.Printf("AI response content: %s", openAIResp.Choices[0].Message.Content)

	// Extract JSON from response (AI might add extra text)
	content := openAIResp.Choices[0].Message.Content
	
	// Try to find JSON object in the response
	startIdx := bytes.IndexByte([]byte(content), '{')
	endIdx := bytes.LastIndexByte([]byte(content), '}')
	
	if startIdx == -1 || endIdx == -1 || startIdx > endIdx {
		return nil, fmt.Errorf("no valid JSON found in AI response: %s", content)
	}
	
	jsonStr := content[startIdx : endIdx+1]
	log.Printf("Extracted JSON: %s", jsonStr)

	var move AIMove
	if err := json.Unmarshal([]byte(jsonStr), &move); err != nil {
		return nil, fmt.Errorf("error unmarshalling move from AI: %v, content: %s", err, jsonStr)
	}

	// Validate move
	if move.Row < 0 || move.Row >= boardSize || move.Col < 0 || move.Col >= boardSize {
		return nil, fmt.Errorf("invalid move coordinates: row=%d, col=%d", move.Row, move.Col)
	}

	if board[move.Row][move.Col] != 0 {
		return nil, fmt.Errorf("invalid move: cell already occupied at row=%d, col=%d", move.Row, move.Col)
	}

	log.Printf("AI move: row=%d, col=%d", move.Row, move.Col)
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
