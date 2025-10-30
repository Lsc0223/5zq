document.addEventListener('DOMContentLoaded', () => {
    const boardElement = document.getElementById('board');
    const statusElement = document.getElementById('status');
    const boardSize = 15;
    let board = Array(boardSize).fill(0).map(() => Array(boardSize).fill(0));
    let currentPlayer = 1; // 1 for black (player), 2 for white (AI)

    function renderBoard() {
        boardElement.innerHTML = '';
        for (let i = 0; i < boardSize; i++) {
            for (let j = 0; j < boardSize; j++) {
                const cell = document.createElement('div');
                cell.classList.add('cell');
                if (board[i][j] === 1) {
                    cell.classList.add('black');
                } else if (board[i][j] === 2) {
                    cell.classList.add('white');
                }
                cell.dataset.row = i;
                cell.dataset.col = j;
                cell.addEventListener('click', handleCellClick);
                boardElement.appendChild(cell);
            }
        }
    }

    async function handleCellClick(event) {
        if (currentPlayer !== 1) return;

        const row = event.target.dataset.row;
        const col = event.target.dataset.col;

        if (board[row][col] !== 0) {
            return;
        }

        board[row][col] = 1;
        renderBoard();
        currentPlayer = 2;
        statusElement.textContent = "AI is thinking...";

        // Get AI move
        const response = await fetch('/api/move', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify({ board }),
        });

        const data = await response.json();

        if (data.error) {
            statusElement.textContent = data.error;
            currentPlayer = 1;
            return;
        }

        board = data.board;
        renderBoard();

        if (data.winner) {
            statusElement.textContent = data.winner === 1 ? "You win!" : "AI wins!";
            currentPlayer = 0;
        } else {
            currentPlayer = 1;
            statusElement.textContent = "Your turn";
        }
    }

    renderBoard();
    statusElement.textContent = "Your turn";
});
