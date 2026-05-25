import asyncio
import json
import os

import websockets
from dotenv import load_dotenv
from mcp.server.fastmcp import FastMCP

load_dotenv()

mcp = FastMCP("MANA-Network")

MANA_WS_URL = os.getenv("MANA_WS_URL")


async def run_mana_ws_command(payload: dict) -> str:
    """Helper function to send a command to MANA and collect the streaming response."""
    try:
        async with websockets.connect(MANA_WS_URL) as ws:
            await ws.send(json.dumps(payload))

            full_response = ""
            while True:
                resp = await ws.recv()
                data = json.loads(resp)

                if data.get("type") == "chunk":
                    full_response += data.get("content", "")
                elif data.get("type") == "error":
                    return f"Error from MANA: {data.get('message')}"
                elif data.get("type") == "end":
                    break

            return full_response.strip()
    except Exception as e:
        return f"Failed to connect to MANA server: {str(e)}"


@mcp.tool()
async def check_mana_status() -> str:
    """
    Check the online/offline status of all AI agents in the MANA network.
    Use this to see which agents are available to talk to.
    """
    payload = {"action": "check_online", "agents": ["all"]}
    return await run_mana_ws_command(payload)


@mcp.tool()
async def wake_mana_agent(agent_slug: str) -> str:
    """
    Start up a dormant agent in the MANA network.
    Pass the exact slug of the agent (e.g., 'airi', 'zephyr').
    """
    payload = {"action": "wake_agent", "agents": [agent_slug]}
    return await run_mana_ws_command(payload)


@mcp.tool()
async def interact_with_mana(message: str) -> str:
    """
    Send a message to the MANA network.
    CRITICAL: You MUST include an @mention for the target agent in your message
    (e.g., '@airi what is the battery status?' or '@all report current tasks').
    """
    payload = {"action": "chat", "message": message}
    return await run_mana_ws_command(payload)


if __name__ == "__main__":
    mcp.run()
