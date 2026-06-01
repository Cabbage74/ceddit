package controller

import (
	"ceddit/service"
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// PostQaStreamHandler handles GET /api/v1/posts/:id/qa/stream
//
// Query parameters:
//   - question (required): the user's question
//   - topK (optional, default 5): number of context chunks to retrieve
//   - maxTokens (optional, default 1024): maximum tokens in the answer
//
// Returns: text/event-stream (SSE) with token-by-token streaming.
func PostQaStreamHandler(c *gin.Context) {
	// Parse post ID.
	postID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid post id"})
		return
	}

	// Parse question.
	question := c.Query("question")
	if question == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "question is required"})
		return
	}

	// Parse optional parameters.
	topK := 5
	if v := c.Query("topK"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 && parsed <= 20 {
			topK = parsed
		}
	}

	maxTokens := 1024
	if v := c.Query("maxTokens"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 && parsed <= 4096 {
			maxTokens = parsed
		}
	}

	// Ensure index exists for this post (lazy build if needed).
	if _, err := service.EnsurePostIndexed(postID); err != nil {
		zap.L().Warn("ensure index failed, continuing without index",
			zap.Int64("post_id", postID),
			zap.Error(err))
	}

	// Set SSE headers.
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no") // Disable nginx buffering.
	c.Writer.WriteHeader(http.StatusOK)

	// Create a context with timeout for the streaming request.
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Minute)
	defer cancel()

	// Start streaming answer.
	contentCh, errCh := service.StreamPostAnswer(ctx, postID, question, topK, maxTokens)

	// Stream tokens as SSE events.
	for {
		select {
		case token, ok := <-contentCh:
			if !ok {
				// Content channel closed — stream complete.
				fmt.Fprintf(c.Writer, "data: [DONE]\n\n")
				c.Writer.Flush()
				return
			}
			// Write token as SSE data event.
			fmt.Fprintf(c.Writer, "data: %s\n\n", token)
			c.Writer.Flush()

		case err, ok := <-errCh:
			if ok && err != nil {
				zap.L().Error("RAG stream error",
					zap.Int64("post_id", postID),
					zap.Error(err))
				fmt.Fprintf(c.Writer, "event: error\ndata: %s\n\n", err.Error())
				c.Writer.Flush()
			}
			return

		case <-ctx.Done():
			zap.L().Warn("RAG stream timeout or client disconnected",
				zap.Int64("post_id", postID))
			return
		}
	}
}
