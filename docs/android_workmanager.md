Android WorkManager MVP for Delta Sync
- Goals
  - Background delta submission to /api/v1/sync
  - Token refresh handling, network/connectivity aware scheduling
  - Conflict reporting to UI for user resolution
- Outline
  - Worker: DeltaSyncWorker
    * Triggers when network available and user is authenticated
    * Reads local delta_queue from Room/SQLite cache, builds /sync payload
    * Posts to server and handles server_changes/client_changes/conflicts
    * Applies server_changes to local DB and marks local changes as synced
  - Work constraints: requireNetwork, charging status optional, backoff policy on failures
- Token handling
  - If 401 occurs, trigger Refresh Token flow, retry once
- Conflict handling
  - If conflicts returned, persist in a local conflicts table and notify UI
- Migration/compatibility
  - Ensure local DB schema aligns with web delta schema for cross-end synchronization
