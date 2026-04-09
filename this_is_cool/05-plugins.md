# Lesson 5: Plugins in Action

Now that we have plugins installed and a marketplace running, let's use them.
We'll create a workspace powered by the **Pokedex** plugins -- a playful example
that demonstrates config validation, cross-value rules, and transforms.

---

## Step 1: Create a Pokedex Workspace

```sh {"name": "create-pokedex-workspace", "interactive": true}
mkdir -p /tmp/zhi-pokedex
cd /tmp/zhi-pokedex
zhi init
```

Now wire it up to use the Pokedex config and transform plugins, with in-memory storage:

```sh {"name": "configure-pokedex", "interactive": true}
cat > /tmp/zhi-pokedex/zhi.yaml << 'YAML'
version: "1"

config:
  provider: pokedex

transform:
  - provider: pokedex

store:
  provider: json
  options:
    path: ./store.json

ui:
  provider: webui
  options:
    addr: "127.0.0.1:9091"
    auto_open: false

components:
  - name: trainer
    description: "Pokemon trainer identity"
    paths: ["pokedex/trainer.name", "pokedex/region"]
    mandatory: true
  - name: pokemon
    description: "Starter Pokemon selection"
    paths: ["pokedex/starter", "pokedex/starter.original"]
    mandatory: true
  - name: goals
    description: "Pokedex completion goals"
    paths: ["pokedex/pokedex.goal"]
    mandatory: true

export:
  templates:
    - name: trainer-card
      template: ./templates/trainer-card.txt.tmpl
      output: ./trainer-card.txt
YAML

echo "Workspace configured!"
```

Create a template for a trainer card:

```sh {"name": "create-trainer-template", "interactive": true}
mkdir -p /tmp/zhi-pokedex/templates
cat > /tmp/zhi-pokedex/templates/trainer-card.txt.tmpl << 'TMPL'
╔══════════════════════════════════╗
║        TRAINER CARD              ║
╠══════════════════════════════════╣
║  Name:    {{ .Get "pokedex/trainer.name" | printf "%-20s" }}║
║  Region:  {{ .Get "pokedex/region" | printf "%-20s" }}║
║  Partner: {{ .Get "pokedex/starter" | printf "%-20s" }}║
║  Goal:    {{ .Get "pokedex/pokedex.goal" | printf "%-20s" }}║
╚══════════════════════════════════╝
TMPL

echo "Template created!"
```

---

## Step 2: Explore the Config Tree

The Pokedex config plugin provides a pre-defined config tree with defaults and validators.

```sh {"name": "list-pokedex", "interactive": true}
cd /tmp/zhi-pokedex && zhi list paths
```

```sh {"name": "get-pokedex-values", "interactive": true}
cd /tmp/zhi-pokedex

echo "=== Trainer ==="
zhi get pokedex/trainer.name
zhi get pokedex/region

echo ""
echo "=== Pokemon ==="
zhi get pokedex/starter

echo ""
echo "=== Goals ==="
zhi get pokedex/pokedex.goal
```

Notice the defaults: Ash from Kanto with a Pikachu, aiming to catch 150 Pokemon.

---

## Step 3: See Transforms in Action

The Pokedex transform plugin **evolves** your starter Pokemon for display.
Watch what happens:

```sh {"name": "see-transform", "interactive": true}
cd /tmp/zhi-pokedex

echo "Setting starter to charmander..."
zhi set pokedex/starter charmander

echo ""
echo "Now reading the value back (through the transform):"
zhi get pokedex/starter
echo ""
echo "The transform evolved charmander → charizard for display!"
echo ""
echo "The original is preserved:"
zhi get pokedex/starter.original
```

The transform plugin:
- **On read:** Evolves `charmander` → `charizard` (final form) and adjusts the
  pokedex goal (+2 for the evolution stages)
- **On save:** Maps back to the base form for storage
- Creates a transient `starter.original` path so you can see both forms

> **Try it yourself:** Set the starter to `bulbasaur` or `squirtle` and see their evolutions!

```sh {"name": "try-evolution", "interactive": true}
cd /tmp/zhi-pokedex

# Your turn! Try another starter:
zhi set pokedex/starter squirtle
echo ""
echo "Starter (evolved):"
zhi get pokedex/starter
echo ""
echo "Original:"
zhi get pokedex/starter.original
```

---

## Step 4: Trigger Validation Rules

The config plugin has several validators. Let's trigger them.

### Invalid region

```sh {"name": "invalid-region", "interactive": true}
cd /tmp/zhi-pokedex

zhi set pokedex/region "middle-earth"
echo ""
zhi validate
```

You should see a **blocking** error -- `middle-earth` isn't a valid Pokemon region.

```sh {"name": "fix-region", "interactive": true}
cd /tmp/zhi-pokedex && zhi set pokedex/region "johto"
```

### Non-classic starter (warning)

```sh {"name": "non-classic-starter", "interactive": true}
cd /tmp/zhi-pokedex

zhi set pokedex/starter "eevee"
echo ""
zhi validate
```

This produces a **warning** (not blocking) -- eevee isn't a classic Kanto starter,
but it's still allowed.

### Cross-value validation

```sh {"name": "cross-validation", "interactive": true}
cd /tmp/zhi-pokedex

# Set a Kanto starter with a non-Kanto region
zhi set pokedex/starter "charmander"
zhi set pokedex/region "johto"
echo ""
zhi validate
```

The config plugin performs **cross-value validation**: it notices that Charmander
is a Kanto starter but you chose Johto as your region, and issues an info message.

### Unrealistic goal

```sh {"name": "high-goal", "interactive": true}
cd /tmp/zhi-pokedex

zhi set pokedex/pokedex.goal 9999
echo ""
zhi validate
```

Warning: there aren't that many Pokemon!

```sh {"name": "fix-goal", "interactive": true}
cd /tmp/zhi-pokedex
zhi set pokedex/pokedex.goal 150
zhi set pokedex/region "kanto"
```

---

## Step 5: Export Your Trainer Card

```sh {"name": "export-trainer-card", "interactive": true}
cd /tmp/zhi-pokedex

zhi export
echo ""
cat trainer-card.txt
```

The exported card shows the **transformed** values (evolved starter, adjusted goal).

> **Try it yourself:** Change the trainer name to yours, pick your favorite starter,
> and export again!

```sh {"name": "customize-and-export", "interactive": true}
cd /tmp/zhi-pokedex

# Customize these values:
zhi set pokedex/trainer.name "Your Name"
zhi set pokedex/starter "bulbasaur"
zhi set pokedex/region "kanto"
zhi set pokedex/pokedex.goal 151

zhi export
echo ""
cat trainer-card.txt
```

---

## Step 6: Inspect Plugin Manifests

Every plugin has a `zhi-plugin.yaml` manifest. Let's look at what's installed.

```sh {"name": "plugin-info", "interactive": true}
# List all installed plugins with details
zhi plugin list

echo ""
echo "=== Plugin directory ==="
ls ~/.zhi/plugins/ 2>/dev/null || echo "(empty -- plugins may be built-in)"
```

---

## Checkpoint

```sh {"name": "check-05"}
cd /tmp/zhi-pokedex
ERRORS=0

# Validation should pass with correct values
zhi set pokedex/trainer.name "Ash" 2>/dev/null
zhi set pokedex/starter "pikachu" 2>/dev/null
zhi set pokedex/region "kanto" 2>/dev/null
zhi set pokedex/pokedex.goal 150 2>/dev/null

VALIDATION=$(zhi validate 2>&1)
BLOCKING=$(echo "$VALIDATION" | grep -c "Blocking" || true)
if [ "$BLOCKING" -eq 0 ]; then
  echo "✓ Validation passes with correct values"
else
  echo "✗ Unexpected blocking errors"
  ERRORS=$((ERRORS + 1))
fi

# Export should work
if zhi export 2>/dev/null && [ -f trainer-card.txt ]; then
  echo "✓ Export generates trainer card"
else
  echo "✗ Export failed"
  ERRORS=$((ERRORS + 1))
fi

# Transform should evolve pikachu
STARTER=$(zhi get pokedex/starter 2>/dev/null)
if echo "$STARTER" | grep -qi "raichu"; then
  echo "✓ Transform evolves pikachu → raichu"
else
  echo "⚠ Transform may not be active (got: $STARTER)"
fi

if [ $ERRORS -eq 0 ]; then
  echo ""
  echo "Plugins are working! On to the final lesson."
fi
```

---

## Cleanup

```sh {"name": "cleanup-05", "interactive": true, "excludeFromRunAll": true}
rm -rf /tmp/zhi-pokedex
echo "Pokedex workspace cleaned up."
```

---

**Next up:** [Lesson 6 - Bring It Together](06-bring-it-together.md) -- we'll build
a custom workspace backed by Vault, completing the full zhi workflow.
