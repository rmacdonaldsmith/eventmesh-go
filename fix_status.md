## Investigation Results

✅ **HTTP API endpoint is working correctly**
- GET /api/v1/topics/{topic}/events works perfectly
- Events are stored with proper incrementing offsets (0,1,2,3...)
- Event IDs increment correctly (demo.events-0, demo.events-1...)

❌ **CLI 'topics info' command has a routing issue**
- Gets 404 error: 'Invalid path, expected /events'  
- Needs investigation of URL construction in httpclient

✅ **Event storage is working properly**
- Offsets increment correctly in the EventLog
- CLI publish output is misleading (shows same offset) but storage is correct

Next: Fix CLI routing issue
