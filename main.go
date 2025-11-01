package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"os"
	"time"
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

type AIMove struct {
	Row int `json:"row"`
	Col int `json:"col"`
}

func main() {
	rand.Seed(time.Now().UnixNano())
	
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "templates/index.html")
	})

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

	// Get AI move with retry logic
	maxRetries := 3
	var aiMove *AIMove
	var err error
	
	for i := 0; i < maxRetries; i++ {
		aiMove, err = getAIMove(req.Board)
		if err == nil && aiMove != nil && req.Board[aiMove.Row][aiMove.Col] == 0 {
			log.Printf("Valid AI move found: row=%d, col=%d", aiMove.Row, aiMove.Col)
			break // Valid move found
		}
		
		if err != nil {
			log.Printf("Retry %d/%d: AI move error: %v", i+1, maxRetries, err)
		} else if aiMove != nil {
			log.Printf("Retry %d/%d: Invalid AI move (occupied cell): row=%d, col=%d", i+1, maxRetries, aiMove.Row, aiMove.Col)
		}
		
		if i == maxRetries-1 {
			// Last retry failed, find a random empty cell
			log.Printf("All retries exhausted, falling back to random empty cell")
			aiMove = findRandomEmptyCell(req.Board)
			if aiMove == nil {
				json.NewEncoder(w).Encode(MoveResponse{Error: "Board is full", Board: req.Board})
				return
			}
			log.Printf("Using random empty cell: row=%d, col=%d", aiMove.Row, aiMove.Col)
		}
	}

	req.Board[aiMove.Row][aiMove.Col] = 2 // AI is player 2 (white)
	
	// Check for winner after AI's move
	if winner := checkWinner(req.Board); winner != 0 {
		json.NewEncoder(w).Encode(MoveResponse{Board: req.Board, Winner: winner})
		return
	}

	json.NewEncoder(w).Encode(MoveResponse{Board: req.Board})
}

func findRandomEmptyCell(board [][]int) *AIMove {
	var emptyCells []AIMove
	for r := 0; r < boardSize; r++ {
		for c := 0; c < boardSize; c++ {
			if board[r][c] == 0 {
				emptyCells = append(emptyCells, AIMove{Row: r, Col: c})
			}
		}
	}
	if len(emptyCells) == 0 {
		return nil
	}
	// Return a random empty cell
	randomIndex := rand.Intn(len(emptyCells))
	return &emptyCells[randomIndex]
}

func getAIMove(board [][]int) (*AIMove, error) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	apiURL := os.Getenv("OPENAI_API_URL")

	if apiKey == "" || apiURL == "" {
		return nil, fmt.Errorf("OPENAI_API_KEY and OPENAI_API_URL must be set")
	}

	log.Printf("Using API URL: %s", apiURL)

	// Build improved prompt
	prompt := "You are a Gomoku AI. The board is 15x15. 1 is black (player), 2 is white (you). Empty cells are 0. It's your turn.\n\n"
	prompt += "CRITICAL RULES:\n"
	prompt += "1. You MUST choose an empty cell (marked as 0)\n"
	prompt += "2. Choosing an occupied cell (1 or 2) will cause an error\n"
	prompt += "3. Reply with ONLY a JSON object: {\"row\": number, \"col\": number}\n"
	prompt += "4. Do NOT include any explanation or text before or after the JSON\n\n"
	
	// List available empty positions
	prompt += "Available empty positions (0-indexed):\n"
	emptyCount := 0
	emptyPositions := ""
	for i, row := range board {
		for j, cell := range row {
			if cell == 0 {
				if emptyCount < 80 { // Limit to avoid token overflow
					emptyPositions += fmt.Sprintf("(%d,%d) ", i, j)
					if (emptyCount+1)%10 == 0 {
						emptyPositions += "\n"
					}
				}
				emptyCount++
			}
		}
	}
	prompt += emptyPositions
	prompt += fmt.Sprintf("\nTotal empty cells: %d\n\n", emptyCount)
	
	prompt += "Current board state:\n"
	for i, row := range board {
		prompt += fmt.Sprintf("Row %2d: %v\n", i, row)
	}
	prompt += "\nYour move (JSON only, must be an empty position):"

	reqBody := map[string]interface{}{
		"model": "gpt-4.1-mini",
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
		"temperature": 0.7,
	}

	reqBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("error marshalling request: %v", err)
	}

	log.Printf("Request body length: %d bytes", len(reqBytes))

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

	if resp.StatusCode != 200 {
		log.Printf("API error response: %s", string(respBody))
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(respBody))
	}

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
		return nil, fmt.Errorf("no choices returned from AI")
	}

	content := openAIResp.Choices[0].Message.Content
	log.Printf("AI response content: %s", content)

	// Extract JSON from response
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

	// Validate move coordinates
	if move.Row < 0 || move.Row >= boardSize || move.Col < 0 || move.Col >= boardSize {
		return nil, fmt.Errorf("invalid move coordinates: row=%d, col=%d", move.Row, move.Col)
	}

	// Validate cell is empty
	if board[move.Row][move.Col] != 0 {
		return nil, fmt.Errorf("invalid move: cell already occupied at row=%d, col=%d", move.Row, move.Col)
	}

	log.Printf("AI move validated: row=%d, col=%d", move.Row, move.Col)
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

	// Check diagonals (top-left to bottom-right)
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

	// Check diagonals (top-right to bottom-left)
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
