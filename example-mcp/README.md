
# Golang example MCP Server

The project binary has a built in way to run an example MCP server, `trivyServer`. 
You can try it out by running:

```bash
go run main.go exampleMCP

```
You can also provide a `--name` and/or `--port` for it. There are example client 
commands in [curl.md](../docs/curl.md). Be sure to adjust port and also the MCP
server URI as necessary. The `exampleMCP` server runs on `8888` and has the base 
level URI `/`.


# Python example MCP Server

A python example MCP server is also provided. It is located in the [trivy](./trivy/)
directory here. It is based on [fastmcp](https://gofastmcp.com/). 

To run it, you first need to activate the Python virtual environment and 
then run the server script. Also provided is a client to try it out directly.


```bash
# Activate the virtual environment
source .venv/bin/activate

# Run the trivy server
python example-mcp/trivy/trivy-server.py

# background the above process or use another terminal
python example-mcp/trivy/trivy-client.py
```

