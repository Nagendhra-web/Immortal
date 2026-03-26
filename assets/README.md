# Assets

## Mascot

Put your robot mascot image here as `mascot.png`.

## Demo GIF

Record a demo GIF showing Immortal in action. Here's how:

### Option 1: VHS (Recommended — Charm.sh tool)

```bash
# Install VHS
go install github.com/charmbracelet/vhs@latest

# Create the recording script
cat > demo.tape << 'EOF'
Output assets/demo.gif
Set Width 800
Set Height 400
Set FontSize 14
Set Theme "Dracula"

Type "# Start our demo app"
Enter
Sleep 500ms

Type "go run demo/fake_server.go &"
Enter
Sleep 2s

Type "# Start Immortal watching it"
Enter
Sleep 500ms

Type "./bin/immortal start --watch-url http://localhost:8089/health --rules demo/heal_rules.json"
Enter
Sleep 3s

Type "# Now break the server"
Enter
Sleep 500ms

Type "curl localhost:8089/break"
Enter
Sleep 2s

Type "# Watch Immortal detect and heal it..."
Enter
Sleep 10s

Type "# Check — server is fixed!"
Enter
Type "curl localhost:8089/health"
Enter
Sleep 2s
EOF

# Record
vhs demo.tape
```

### Option 2: asciinema + agg

```bash
# Record
asciinema rec demo.cast

# Convert to GIF
agg demo.cast assets/demo.gif --theme dracula
```

### Option 3: Screen recorder

Use any screen recorder (OBS, Kap, ScreenToGif) and record:

1. Terminal split screen
2. Left: `immortal start --watch-url http://localhost:8089/health --rules demo/heal_rules.json`
3. Right: `curl localhost:8089/break` then watch it heal
4. Export as GIF, save as `assets/demo.gif`

### What the demo should show:

```
1. Server starts healthy          (2 seconds)
2. Immortal starts watching       (animated startup)
3. Server breaks (curl /break)    (1 second)
4. Immortal detects + heals       (auto — [HEAL] message appears)
5. Server is healthy again        (curl /health → 200 OK)

Total: ~15 seconds
```
