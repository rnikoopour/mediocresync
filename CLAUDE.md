# Tests
- ALWAYS run `make test` after making any code changes. Do not commit or consider a task complete until all tests pass.

# Real-time Updates
- Use SSE for all real-time UI updates. Do not add polling (`refetchInterval`).
- SSE invalidates React Query cache when events arrive; queries fetch the final state once.
