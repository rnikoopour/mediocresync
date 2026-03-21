# Tests
- Run `make test` after making changes to the codebase.

# Real-time Updates
- Use SSE for all real-time UI updates. Do not add polling (`refetchInterval`).
- SSE invalidates React Query cache when events arrive; queries fetch the final state once.
