#!/bin/bash
set -euo pipefail

# Build the documentation site
# Outputs to docs/site/

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
SITE_DIR="$PROJECT_ROOT/docs/site"
TEMPLATE="$PROJECT_ROOT/docs/template.html"

mkdir -p "$SITE_DIR"
mkdir -p "$SITE_DIR/transcripts"

echo "Building documentation site..."

# Build index.html from README.md
echo "  Building index.html..."
pandoc "$PROJECT_ROOT/README.md" \
    --template="$TEMPLATE" \
    --metadata title="Home" \
    --metadata is_home=true \
    -o "$SITE_DIR/index.html"

# Build transcripts list page
echo "  Building transcripts.html..."
TRANSCRIPT_LIST=""
# Sort by PR number in descending order (newest first)
# Extract filenames, sort numerically by PR number (first field), then reconstruct full paths
for filename in $(ls -1 "$PROJECT_ROOT"/transcripts/*.html 2>/dev/null | xargs -n1 basename | sort -t'-' -k1 -rn); do
    f="$PROJECT_ROOT/transcripts/$filename"
    if [ -f "$f" ]; then
        # Extract PR number and description from filename like "12-room-scoped-message-routing.html"
        pr_num=$(echo "$filename" | cut -d'-' -f1)
        title=$(echo "$filename" | sed 's/^[0-9]*-//; s/\.html$//; s/-/ /g')
        # Capitalize first letter of each word
        title=$(echo "$title" | awk '{for(i=1;i<=NF;i++) $i=toupper(substr($i,1,1)) tolower(substr($i,2))}1')
        TRANSCRIPT_LIST="$TRANSCRIPT_LIST<li class=\"transcript-row\"><a href=\"https://github.com/llimllib/hatchat/pull/$pr_num\" class=\"transcript-number\">PR #$pr_num</a>: <a href=\"transcripts/$filename\" class=\"transcript-link\"><span class=\"transcript-title\">$title</span></a></li>"
        
        # Copy transcript to site
        cp "$f" "$SITE_DIR/transcripts/"
    fi
done

# Create transcripts page content
cat > /tmp/transcripts.md << EOF
# Chat Transcripts

These are the AI pair programming session transcripts from developing Hatchat.

<ul class="transcript-list">
$TRANSCRIPT_LIST
</ul>
EOF

pandoc /tmp/transcripts.md \
    --template="$TEMPLATE" \
    --metadata title="Transcripts" \
    --metadata is_transcripts=true \
    -o "$SITE_DIR/transcripts.html"

# Build protocol docs page (embed the json-schema-for-humans output)
echo "  Building protocol.html..."

# Generate the schema docs directly to site directory with CSS/JS
echo "    Regenerating protocol schema docs..."
uvx --from json-schema-for-humans generate-schema-doc \
  --copy-css --copy-js --expand-buttons \
  "$PROJECT_ROOT/schema/protocol.json" "$SITE_DIR/protocol-schema.html"

# Create a page that embeds the protocol docs
cat > /tmp/protocol.md << EOF
# WebSocket Protocol Documentation

<iframe src="protocol-schema.html" class="protocol-frame" title="Protocol Schema Documentation"></iframe>
EOF

pandoc /tmp/protocol.md \
    --template="$TEMPLATE" \
    --metadata title="Protocol" \
    --metadata is_protocol=true \
    -o "$SITE_DIR/protocol.html"

# Build REST API docs page
echo "  Building rest-api.html..."

# Copy the OpenAPI spec to site directory
cp "$PROJECT_ROOT/docs/openapi.yaml" "$SITE_DIR/"

# Create a page that uses Swagger UI to display the REST API
# We'll use a simple embedded view with Swagger UI from CDN
cat > "$SITE_DIR/rest-api-schema.html" << 'SWAGGEREOF'
<!DOCTYPE html>
<html>
<head>
  <title>Hatchat REST API</title>
  <link rel="stylesheet" type="text/css" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css" />
  <style>
    body { margin: 0; padding: 0; }
    .swagger-ui .topbar { display: none; }
    .swagger-ui .info { margin: 20px 0; }
  </style>
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
  <script>
    window.onload = function() {
      SwaggerUIBundle({
        url: "openapi.yaml",
        dom_id: '#swagger-ui',
        presets: [
          SwaggerUIBundle.presets.apis,
          SwaggerUIBundle.SwaggerUIStandalonePreset
        ],
        layout: "BaseLayout"
      })
    }
  </script>
</body>
</html>
SWAGGEREOF

# Create a page that embeds the REST API docs
cat > /tmp/rest-api.md << EOF
# REST API Documentation

<iframe src="rest-api-schema.html" class="protocol-frame" title="REST API Documentation"></iframe>
EOF

pandoc /tmp/rest-api.md \
    --template="$TEMPLATE" \
    --metadata title="REST API" \
    --metadata is_rest_api=true \
    -o "$SITE_DIR/rest-api.html"

echo "Done! Site built at $SITE_DIR"
