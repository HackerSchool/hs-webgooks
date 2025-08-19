# hs-webgooks
WebHook proxy from Vikunja to Hacker League API made in Go

## Features

- **Task Creation Notifications**: Automatically sends Discord messages when new tasks are created in Vikunja *(Discord features currently dormant)*
- **Task Completion Tracking**: Detects when tasks are completed and sends data to Hacker League API
- **Project-based Routing**: Routes notifications to different Discord channels based on project prefixes *(Discord features currently dormant)*
- **Points System Integration**: Awards 30 points to members when they complete tasks

## Environment Variables

Create a `.env` file with the following variables:

```bash
# Discord Bot Token (currently not used - Discord features are dormant)
DISCORD_BOT_TOKEN=your_discord_bot_token_here

# Hacker League API URL (optional - defaults to localhost:3000)
HACKER_LEAGUE_API_URL=http://localhost:3000/tasks
```

## How it works

1. **Vikunja Integration**: Listens for webhooks from Vikunja on port 4030
2. **Event Handling**: 
   - `task.created`: Sends Discord notifications *(currently dormant)*
   - `task.updated`: Detects when `done = true` and sends data to Hacker League API, awarding 30 points
3. **Project Routing**: Uses `channels.json` to route notifications to appropriate Discord channels *(currently dormant)*
4. **Points System**: Automatically awards 30 PCC points when tasks are completed
5. **Mock API Server**: Includes a local mock API server on port 3000 for testing

## API Integration

When a task is completed, the service sends a POST request to your Hacker League API with:

```json
{
  "time": "2025-08-19T12:00:00Z",
  "assignees": ["Alice Developer"],
  "description": "Fixed authentication issue",
  "title": "Fix Login Bug",
  "project": "hsdev",
  "points": 30,
  "task_id": 123,
  "list_id": 456
}
```

## Testing with Real Vikunja Data Structure

The service now handles Vikunja's `task.updated` event with the real data structure. To test locally:

```powershell
$body = @{
  event_name = "task.updated"
  time = "2025-08-19T12:00:00Z"
  data = @{
    doer = @{
      id = 18
      name = "Armando Gonçalves"
      username = "armandoski"
    }
    task = @{
      id = 123
      title = "Fix Login Bug"
      description = "Fixed authentication issue"
      done = $true
      done_at = "2025-08-19T12:00:00Z"
      identifier = "hsdev-123"  # Must contain project prefix (hsdev-, hsmkt-, etc.)
      project_id = 60
      assignees = @(
        @{ id = 1; name = "Alice Developer" }
      )
      created_by = @{ id = 1; name = "Alice Developer" }
    }
  }
} | ConvertTo-Json -Depth 6

Invoke-RestMethod -Method Post -Uri http://localhost:4030/ -ContentType "application/json" -Body $body
```

**Important Notes**: 
- The service only processes tasks when `done = true` (task completed)
- **Task identifiers must contain project prefix** (e.g., `hsdev-123`, `hsmkt-456`) for proper routing
- Discord features are currently dormant and will be re-enabled later

## What Gets Sent to Your API

When a task is completed, your API receives:

```json
{
  "time": "2025-08-19T12:00:00Z",
  "assignees": ["Alice Developer"],
  "description": "Fixed authentication issue",
  "title": "Fix Login Bug",
  "project": "hsdev",
  "points": 30,
  "task_id": 123,
  "list_id": 60
}
```

## Current Status

- ✅ **Webhook Processing**: Fully functional
- ✅ **Points System**: Awards 30 points automatically
- ✅ **Mock API Server**: Running on port 3000 for testing
- ⏸️ **Discord Features**: Currently dormant (commented out in code)
- 🔄 **Vikunja Integration**: Ready for production use

## Future Enhancements

- Re-enable Discord notifications for task completion
- Add support for more Vikunja events
- Implement webhook authentication
- Add database logging for points history
