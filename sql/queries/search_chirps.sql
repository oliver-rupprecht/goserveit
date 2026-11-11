-- name: SearchChirps :one
SELECT * FROM chirps WHERE chirps.id = $1; 
