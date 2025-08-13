# GPM CLI Installation Web Page

This document outlines what should be served at `https://gpm.sh/install.sh` and related endpoints.

## Required Endpoints

### 1. `https://gpm.sh/install.sh`
- **Purpose**: Serve the installation script directly
- **Content-Type**: `text/plain` or `application/x-sh`
- **File**: Serve the `install.sh` script from this repository

### 2. `https://gpm.sh/install` (Optional Web Page)
- **Purpose**: Human-friendly installation page
- **Content-Type**: `text/html`
- **Content**: Installation instructions and copy-paste commands

## Web Server Configuration

### Nginx Example
```nginx
server {
    listen 80;
    listen 443 ssl;
    server_name gpm.sh;

    # Serve installation script
    location = /install.sh {
        alias /path/to/gpm-cli/install.sh;
        add_header Content-Type text/plain;
        add_header Cache-Control "public, max-age=3600";
    }

    # Optional: Serve installation page
    location = /install {
        alias /path/to/install.html;
        add_header Content-Type text/html;
    }

    # Redirect root to main site or installation page
    location = / {
        return 301 https://github.com/gpm-sh/gpm-cli;
    }
}
```

### Apache Example
```apache
<VirtualHost *:80>
<VirtualHost *:443>
    ServerName gpm.sh
    DocumentRoot /var/www/gpm

    # Serve installation script
    Alias /install.sh /path/to/gpm-cli/install.sh
    <Location "/install.sh">
        Header set Content-Type "text/plain"
        Header set Cache-Control "public, max-age=3600"
    </Location>

    # Redirect root
    RedirectMatch 301 ^/$ https://github.com/gpm-sh/gpm-cli
</VirtualHost>
```

## HTML Installation Page (Optional)

Create a simple HTML page at `https://gpm.sh/install`:

```html
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Install GPM CLI - Unity Package Manager</title>
    <style>
        body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; }
        .container { max-width: 800px; margin: 0 auto; padding: 2rem; }
        .code { background: #f5f5f5; padding: 1rem; border-radius: 4px; font-family: monospace; }
        .copy-btn { margin-left: 1rem; padding: 0.5rem 1rem; background: #007acc; color: white; border: none; border-radius: 4px; cursor: pointer; }
    </style>
</head>
<body>
    <div class="container">
        <h1>Install GPM CLI</h1>
        <p>GPM CLI is a package manager for Unity projects. Install it with a single command:</p>
        
        <h2>Quick Install</h2>
        <div class="code">
            curl -fsSL https://gpm.sh/install.sh | bash
            <button class="copy-btn" onclick="copyToClipboard('curl -fsSL https://gpm.sh/install.sh | bash')">Copy</button>
        </div>
        
        <h2>Or with wget</h2>
        <div class="code">
            wget -qO- https://gpm.sh/install.sh | bash
            <button class="copy-btn" onclick="copyToClipboard('wget -qO- https://gpm.sh/install.sh | bash')">Copy</button>
        </div>

        <h2>Manual Download</h2>
        <p>Download pre-built binaries from <a href="https://github.com/gpm-sh/gpm-cli/releases">GitHub Releases</a>.</p>

        <h2>After Installation</h2>
        <div class="code">
            gpm register  # Create account<br>
            gpm login     # Login<br>
            gpm --help    # Show help
        </div>
    </div>

    <script>
        function copyToClipboard(text) {
            navigator.clipboard.writeText(text).then(() => {
                // Could show a toast notification here
                console.log('Copied to clipboard');
            });
        }
    </script>
</body>
</html>
```

## Deployment Checklist

1. ✅ Upload `install.sh` to your web server
2. ✅ Configure web server to serve script at `/install.sh`
3. ✅ Set appropriate headers (`Content-Type`, `Cache-Control`)
4. ✅ Test the installation command
5. ✅ Optional: Create installation web page at `/install`
6. ✅ Update DNS to point `gpm.sh` to your server

## Testing

Test the installation from various platforms:

```bash
# Test the script download
curl -fsSL https://gpm.sh/install.sh | head -10

# Test actual installation (in a container/VM)
curl -fsSL https://gpm.sh/install.sh | bash

# Test with different options
curl -fsSL https://gpm.sh/install.sh | bash -s -- --help
curl -fsSL https://gpm.sh/install.sh | bash -s -- -v v0.1.0-alpha.2
```

## Security Considerations

1. **HTTPS Only**: Always serve over HTTPS to prevent MITM attacks
2. **Script Integrity**: Consider adding checksums or signatures
3. **Domain Security**: Ensure `gpm.sh` domain is properly secured
4. **Rate Limiting**: Implement rate limiting to prevent abuse
5. **Logging**: Log installation attempts for monitoring

## Analytics (Optional)

Track installation metrics:
- Number of installations per day/platform
- Popular installation methods
- Geographic distribution
- Version preferences

This can be done via web server logs or simple analytics embedded in the script.
