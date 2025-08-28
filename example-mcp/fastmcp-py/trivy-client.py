import asyncio
from fastmcp import FastMCP, Client


# TODO get the figgin port from env
client = Client("http://localhost:8888/mcp")
# client = Client("http://localhost:3000/mcp")

async def initialize_result():
    async with client:
        print(f'initialize_result: {client.initialize_result}')

async def tools_list():
    async with client:
        print(f'tools_list: {client.list_tools}')


async def call_tool(method:str, args: dict):
    async with client:
        result = await client.call_tool(method, args)
        print(f'call_tool {method}({args}): {result}')

async def ping():
    async with client:
        result = await client.ping()
        print(f'ping result: {result}')


# asyncio.run( initialize_result() )
# asyncio.run( tools_list() )
asyncio.run(call_tool( "add", {"a": 6, "b": 2} ))

asyncio.run( ping() )


