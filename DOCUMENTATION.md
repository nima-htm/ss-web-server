# SS-Web-Server - Go-based Nginx Alternative

SS-Web-Server is a lightweight, Go-based web server with reverse proxy capabilities designed as an alternative to nginx. It can be easily integrated into Docker projects to serve static files and proxy requests to backend services.

## Overview

This is a lightweight web server written in Go that functions as a reverse proxy server, similar to nginx. It can forward requests to backend servers (like FastAPI or Django applications) and serve static files.

## Features

- **Reverse Proxy**: Forward requests to backend services (e.g., FastAPI, Django, Flask)
- **Static File Serving**: Efficiently serve static assets
- **Load Balancing**: Round-robin distribution across multiple backend instances
- **Configuration**: YAML-based configuration similar to nginx
- **Lightweight**: Small Docker image size, minimal resource usage

## Architecture

The project is organized into several modules:

### 1. Internal Modules

- **internal/config**: Handles configuration loading and validation
- **internal/proxy**: Implements reverse proxy functionality
- **internal/static**: Handles static file serving
- **internal/server**: Main server orchestration

### 2. Configuration

The server uses a YAML-based configuration similar to nginx. The configuration file (default.conf) contains:

- Server blocks (`servers`) - Define listening ports and virtual hosts
- Location blocks (`locations`) - Define routing rules
- Upstream blocks (`upstreams`) - Define backend server pools

Example configuration:

```yaml
servers:
  - listen: ":8080"
    server_name: "localhost"
    locations:
      - path: "/api/"
        proxy_pass: "backend"
        proxy_set:
          X-Real-IP: "$remote_addr"
        proxy_pass_headers:
          - "Authorization"
          - "Content-Type"
          - "Accept"
      - path: "/"
        root: "./static"
        index: "index.html"

upstreams:
  - name: "backend"
    servers:
      - "http://localhost:8000"
      - "http://localhost:8001"
```

## How to Use

### Running the Server

```bash
go run main.go
```By default, the server looks for `default.conf` in the current directory. You can specify a different config file using:

```bash
go run main.go -config myconfig.conf
```

### Testing the Application

1. Start the server:
   ```bash
   go run main.go
   ```

2. The server will start on port 8080 (according to default.conf)

3. To test static file serving, visit:
   - `http://localhost:8080/` - Serves the index.html file from the static directory

4. To test reverse proxy functionality, you would need to:
   - Have a backend server running on one of the configured upstream servers (default is localhost:800 and localhost:8001)
   - Make requests to the /api/ path which will be forwarded to the backend:
     - `http://localhost:8080/api/users` (would forward to your backend)

### Example Use Case

For a typical web application with a frontend and backend:

1. Backend: FastAPI/Django app running on `http://localhost:8000`
2. Frontend: Static files in `./static` directory
3. Proxy server: Listens on port 8080, serves static files and forwards API requests to backend

The configuration would route:

- `/api/*` requests to the backend servers
- All other requests to static file serving

## Using in Your Own Projects (Nginx Replacement)

SS-Web-Server can be used as a drop-in replacement for nginx in your projects. Here's how to integrate it into a typical Flask application:

<!-- ### Example Dockerfile for a Flask App using SS-Web-Server

```Dockerfile

FROM python:3.12-slim AS backend
WORKDIR /app
COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt
COPY app.py .

FROM nimahtm/ss-web-server:latest AS web-server

FROM python:3.12-slim AS final
WORKDIR /app
RUN pip install --no-cache-dir Flask==2.3 Werkzeug==2.3.7
COPY app.py .
COPY --from=backend /usr/local/lib/python3.*/site-packages /usr/local/lib/python3.12/site-packages

RUN mkdir -p /var/www/html
RUN echo "<html><body><h1>Frontend App</h1><p>This is served by ss-web-server</p></body></html>" > /var/www/html/index.html


COPY ss-web-server.conf /etc/ss-web-server/ss-web-server.conf
EXPOSE 80

CMD ["sh", "-c", "python app.py & ss-web-server -config /etc/ss-web-server/ss-web-server.conf"] -->
```

### Example Configuration for Flask App

```yaml
servers:
  - listen: ":80"
    server_name: "localhost"
    locations:
      - path: "/api/"
        proxy_pass: "backend"
        proxy_set:
          X-Real-IP: "$remote_addr"
        proxy_pass_headers:
          - "Authorization"
          - "Content-Type"
          - "Accept"
      - path: "/"
        root: "/var/www/html"
        index: "index.html"

upstreams:
  - name: "backend"
    servers:
      - "http://localhost:5000"
```

## Docker Hub Distribution

The image is available on Docker Hub as `nimahtm/ss-web-server:latest` for easy integration into your projects.

To build the Docker image locally:

```bash
docker build -t ss-web-server .
```

## Code Structure

### internal/config/config.go

- Defines configuration structures
- Loads and validates YAML configuration
- Provides helper methods for configuration access

### internal/proxy/proxy.go

- Implements the reverse proxy functionality
- Handles request forwarding to upstream servers
- Implements load balancing algorithms
- Manages request/response headers

### internal/static/static.go
- Implements static file serving
- Handles directory indexing
- Provides MIME type detection

### internal/server/server.go
- Main server orchestration
- Creates HTTP servers based on configuration
- Manages request routing to appropriate handlers

### main.go
- Entry point of the application
- Loads configuration and starts the server

## Extending the Server

The modular design allows for easy extension:

1. Add new handler types in the internal packages
2. Modify the configuration structure to support new features
3. Update the server logic to use new handlers based on configuration

## Troubleshooting

### Common Issues and Solutions

#### 1. Configuration File Errors
- **Problem:** Server fails to start with a configuration error.
- **Solution:**
  - Ensure the `default.conf` file is in the correct directory.
  - Validate the YAML syntax using an online YAML validator.
  - Check for duplicate `server_name` values or missing `listen` directives.

#### 2. Backend Server Connectivity Issues
- **Problem:** Requests to `/api/` return a `502 Bad Gateway` error.
- **Solution:**
  - Verify that the backend servers defined in the `upstreams` block are running.
  - Check the network connectivity between the web server and the backend servers.
  - Ensure the `proxy_pass` URLs are correct and reachable.

#### 3. Static File Not Found Errors
- **Problem:** Requests to `/` return a `404 Not Found` error.
- **Solution:**
  - Ensure the `root` directory exists and contains the `index.html` file.
  - Verify the `root` path in the configuration file is correct.

#### 4. Dynamic Configuration Reload Issues
- **Problem:** Changes to the configuration file are not applied.
- **Solution:**
  - Ensure the `fsnotify` library is correctly watching the configuration file.
  - Check the logs for any errors during the reload process.

#### 5. Upstream Server Health Checks
- **Problem:** Requests are routed to unhealthy backend servers.
- **Solution:**
  - Verify the health check logic in the `ProxyHandler`.
  - Ensure the backend servers return a `2xx` status code for health check requests.
