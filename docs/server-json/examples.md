# Server JSON Examples

## Basic Server with NPM Package

```json
{
  "name": "io.modelcontextprotocol/brave-search",
  "description": "MCP server for Brave Search API integration",
  "status": "active",
  "repository": {
    "url": "https://github.com/modelcontextprotocol/servers",
    "source": "github",
    "id": "abc123de-f456-7890-ghij-klmnopqrstuv"
  },
  "version_detail": {
    "version": "1.0.2"
  },
  "packages": [
    {
      "registry_name": "npm",
      "name": "@modelcontextprotocol/server-brave-search",
      "version": "1.0.2",
      "environment_variables": [
        {
          "name": "BRAVE_API_KEY",
          "description": "Brave Search API Key",
          "is_required": true,
          "is_secret": true
        }
      ]
    }
  ]
}
```

## Constant (fixed) arguments needed to start the MCP server

Suppose your MCP server application requires a `mcp start` CLI arguments to start in MCP server mode. Express these as positional arguments like this:

```json
{
  "name": "Knapcode.SampleMcpServer",
  "description": "Sample NuGet MCP server for a random number and random weather",
  "version_detail": {
    "version": "0.4.0-beta"
  },
  "packages": [
    {
      "registry_name": "nuget",
      "name": "Knapcode.SampleMcpServer",
      "version": "0.4.0-beta",
      "package_arguments": [
        {
          "type": "positional",
          "value": "mcp"
        },
        {
          "type": "positional",
          "value": "start"
        }
      ]
    }
  ]
}
```

This will essentially instruct the MCP client to execute `dnx Knapcode.SampleMcpServer@0.4.0-beta -- mcp start` instead of the default `dnx Knapcode.SampleMcpServer@0.4.0-beta` (when no `package_arguments` are provided).

## Filesystem Server with Multiple Packages

```json
{
  "name": "io.modelcontextprotocol/filesystem",
  "description": "Node.js server implementing Model Context Protocol (MCP) for filesystem operations.",
  "status": "active",
  "repository": {
    "url": "https://github.com/modelcontextprotocol/servers",
    "source": "github",
    "id": "b94b5f7e-c7c6-d760-2c78-a5e9b8a5b8c9"
  },
  "version_detail": {
    "version": "1.0.2"
  },
  "packages": [
    {
      "registry_name": "npm",
      "name": "@modelcontextprotocol/server-filesystem",
      "version": "1.0.2",
      "package_arguments": [
        {
          "type": "positional",
          "value_hint": "target_dir",
          "description": "Path to access",
          "default": "/Users/username/Desktop",
          "is_required": true,
          "is_repeated": true
        }
      ],
      "environment_variables": [
        {
          "name": "LOG_LEVEL",
          "description": "Logging level (debug, info, warn, error)",
          "default": "info"
        }
      ]
    },
    {
      "registry_name": "docker",
      "name": "mcp/filesystem",
      "version": "1.0.2",
      "runtime_arguments": [
        {
          "type": "named",
          "description": "Mount a volume into the container",
          "name": "--mount",
          "value": "type=bind,src={source_path},dst={target_path}",
          "is_required": true,
          "is_repeated": true,
          "variables": {
            "source_path": {
              "description": "Source path on host",
              "format": "filepath",
              "is_required": true
            },
            "target_path": {
              "description": "Path to mount in the container. It should be rooted in `/project` directory.",
              "is_required": true,
              "default": "/project"
            }
          }
        }
      ],
      "package_arguments": [
        {
          "type": "positional",
          "value_hint": "target_dir",
          "value": "/project"
        }
      ],
      "environment_variables": [
        {
          "name": "LOG_LEVEL",
          "description": "Logging level (debug, info, warn, error)",
          "default": "info"
        }
      ]
    }
  ]
}
```

## Remote Server Example

```json
{
  "name": "Remote Filesystem Server",
  "description": "Cloud-hosted MCP filesystem server",
  "repository": {
    "url": "https://github.com/example/remote-fs",
    "source": "github",
    "id": "xyz789ab-cdef-0123-4567-890ghijklmno"
  },
  "version_detail": {
    "version": "2.0.0"
  },
  "remotes": [
    {
      "transport_type": "sse",
      "url": "https://mcp-fs.example.com/sse"
    }
  ]
}
```

## Python Package Example

```json
{
  "name": "weather-mcp-server",
  "description": "Python MCP server for weather data access",
  "repository": {
    "url": "https://github.com/example/weather-mcp",
    "source": "github",
    "id": "def456gh-ijkl-7890-mnop-qrstuvwxyz12"
  },
  "version_detail": {
    "version": "0.5.0"
  },
  "packages": [
    {
      "registry_name": "pypi",
      "name": "weather-mcp-server",
      "version": "0.5.0",
      "runtime_hint": "uvx",
      "environment_variables": [
        {
          "name": "WEATHER_API_KEY",
          "description": "API key for weather service",
          "is_required": true,
          "is_secret": true
        },
        {
          "name": "WEATHER_UNITS",
          "description": "Temperature units (celsius, fahrenheit)",
          "default": "celsius"
        }
      ]
    }
  ]
}
```

## NuGet (.NET) Package Example

The `dnx` tool ships with the .NET 10 SDK, starting with Preview 6.

```json
{
  "name": "Knapcode.SampleMcpServer",
  "description": "Sample NuGet MCP server for a random number and random weather",
  "repository": {
    "url": "https://github.com/joelverhagen/Knapcode.SampleMcpServer",
    "source": "github",
    "id": "example-nuget-id-0000-1111-222222222222"
  },
  "version_detail": {
    "version": "0.5.0"
  },
  "packages": [
    {
      "registry_name": "nuget",
      "name": "Knapcode.SampleMcpServer",
      "version": "0.5.0",
      "runtime_hint": "dnx",
      "environment_variables": [
        {
          "name": "WEATHER_CHOICES",
          "description": "Comma separated list of weather descriptions to randomly select.",
          "is_required": true,
          "is_secret": false
        }
      ]
    }
  ]
}
```

## Complex Docker Server with Multiple Arguments

```json
{
  "name": "mcp-database-manager",
  "description": "MCP server for database operations with support for multiple database types",
  "repository": {
    "url": "https://github.com/example/mcp-database",
    "source": "github",
    "id": "ghi789jk-lmno-1234-pqrs-tuvwxyz56789"
  },
  "version_detail": {
    "version": "3.1.0"
  },
  "packages": [
    {
      "registry_name": "docker",
      "name": "mcp/database-manager",
      "version": "3.1.0",
      "runtime_arguments": [
        {
          "type": "named",
          "name": "--network",
          "value": "host",
          "description": "Use host network mode"
        },
        {
          "type": "named",
          "name": "-e",
          "value": "DB_TYPE={db_type}",
          "description": "Database type to connect to",
          "is_repeated": true,
          "variables": {
            "db_type": {
              "description": "Type of database",
              "choices": ["postgres", "mysql", "mongodb", "redis"],
              "is_required": true
            }
          }
        }
      ],
      "package_arguments": [
        {
          "type": "named",
          "name": "--host",
          "description": "Database host",
          "default": "localhost",
          "is_required": true
        },
        {
          "type": "named",
          "name": "--port",
          "description": "Database port",
          "format": "number"
        },
        {
          "type": "positional",
          "value_hint": "database_name",
          "description": "Name of the database to connect to",
          "is_required": true
        }
      ],
      "environment_variables": [
        {
          "name": "DB_USERNAME",
          "description": "Database username",
          "is_required": true
        },
        {
          "name": "DB_PASSWORD",
          "description": "Database password",
          "is_required": true,
          "is_secret": true
        },
        {
          "name": "SSL_MODE",
          "description": "SSL connection mode",
          "default": "prefer",
          "choices": ["disable", "prefer", "require"]
        }
      ]
    }
  ]
}
```

## Server with Remote and Package Options

```json
{
  "name": "hybrid-mcp-server",
  "description": "MCP server available as both local package and remote service",
  "repository": {
    "url": "https://github.com/example/hybrid-mcp",
    "source": "github",
    "id": "klm012no-pqrs-3456-tuvw-xyz789abcdef"
  },
  "version_detail": {
    "version": "1.5.0"
  },
  "packages": [
    {
      "registry_name": "npm",
      "name": "@example/hybrid-mcp-server",
      "version": "1.5.0",
      "runtime_hint": "npx",
      "package_arguments": [
        {
          "type": "named",
          "name": "--mode",
          "description": "Operation mode",
          "default": "local",
          "choices": ["local", "cached", "proxy"]
        }
      ]
    }
  ],
  "remotes": [
    {
      "transport_type": "sse",
      "url": "https://hybrid-mcp.example.com/sse",
      "headers": [
        {
          "name": "X-API-Key",
          "description": "API key for authentication",
          "is_required": true,
          "is_secret": true
        },
        {
          "name": "X-Region",
          "description": "Service region",
          "default": "us-east-1",
          "choices": ["us-east-1", "eu-west-1", "ap-southeast-1"]
        }
      ]
    },
    {
      "transport_type": "streamable",
      "url": "https://hybrid-mcp.example.com/stream"
    }
  ]
}
```

## MCP Bundle (MCPB) Package Example

```json
{
  "name": "io.modelcontextprotocol/text-editor",
  "description": "MCP Bundle server for advanced text editing capabilities",
  "repository": {
    "url": "https://github.com/modelcontextprotocol/text-editor-mcpb",
    "source": "github",
    "id": "mcpb-123ab-cdef4-56789-012ghi-jklmnopqrstu"
  },
  "version_detail": {
    "version": "1.0.2"
  },
  "packages": [
    {
      "registry_name": "mcpb",
      "name": "https://github.com/modelcontextprotocol/text-editor-mcpb/releases/download/v1.0.2/text-editor.mcpb",
      "version": "1.0.2",
      "file_hashes": {
        "sha-256": "fe333e598595000ae021bd27117db32ec69af6987f507ba7a63c90638ff633ce"
      }
    }
  ]
}
```

This example shows an MCPB (MCP Bundle) package that:
- Is hosted on GitHub Releases (an allowlisted provider)
- Includes a SHA-256 hash for integrity verification
- Can be downloaded and executed directly by MCP clients that support MCPB

## Deprecated Server Example

```json
{
  "name": "io.legacy/old-weather-server",
  "description": "Legacy weather server - DEPRECATED: Use weather-v2 instead for new projects",
  "status": "deprecated",
  "repository": {
    "url": "https://github.com/example/old-weather",
    "source": "github",
    "id": "legacy-abc123-def456-789012-345678-901234567890"
  },
  "version_detail": {
    "version": "0.9.5"
  },
  "packages": [
    {
      "registry_name": "npm",
      "name": "@legacy/old-weather-server",
      "version": "0.9.5",
      "environment_variables": [
        {
          "name": "WEATHER_API_KEY",
          "description": "Weather API key",
          "is_required": true,
          "is_secret": true
        }
      ]
    }
  ]
}
```