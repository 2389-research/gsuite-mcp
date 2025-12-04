# ABOUTME: MCP server implementation for GSuite APIs
# ABOUTME: Exposes Gmail, Calendar, and People services as MCP tools

import logging
from typing import Any, Dict, List
from pathlib import Path

from mcp.server import Server
from mcp.types import Tool, TextContent

from gsuite_mcp.auth.oauth import OAuth2Authenticator
from gsuite_mcp.services.gmail import GmailService
from gsuite_mcp.services.calendar import CalendarService
from gsuite_mcp.services.people import PeopleService

logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)


class GSuiteMCPServer:
    """MCP Server for Google Workspace APIs."""

    def __init__(
        self,
        credentials_path: Path = Path("credentials.json"),
        token_path: Path = Path("token.json"),
    ):
        """Initialize GSuite MCP Server.

        Args:
            credentials_path: Path to OAuth credentials
            token_path: Path to cached token
        """
        self.name = "gsuite-mcp"
        self.version = "0.1.0"

        # Initialize OAuth
        self.authenticator = OAuth2Authenticator(
            credentials_path=credentials_path,
            token_path=token_path,
        )

        # Get credentials
        creds = self.authenticator.get_credentials()

        # Initialize services
        self.gmail_service = GmailService(creds)
        self.calendar_service = CalendarService(creds)
        self.people_service = PeopleService(creds)

        # Initialize MCP server
        self.server = Server(self.name)
        self._register_tools()

    def _register_tools(self) -> None:
        """Register all MCP tools."""

        # Gmail tools
        @self.server.list_tools()
        async def list_tools() -> List[Tool]:
            """List available tools."""
            return [
                # Gmail - Messages
                Tool(
                    name="gmail_list_messages",
                    description="List Gmail messages with optional filters",
                    inputSchema={
                        "type": "object",
                        "properties": {
                            "query": {"type": "string", "description": "Gmail search query"},
                            "max_results": {"type": "integer", "default": 100},
                        },
                    },
                ),
                Tool(
                    name="gmail_get_message",
                    description="Get a specific Gmail message by ID",
                    inputSchema={
                        "type": "object",
                        "properties": {
                            "message_id": {"type": "string"},
                        },
                        "required": ["message_id"],
                    },
                ),
                Tool(
                    name="gmail_send_message",
                    description="Send an email",
                    inputSchema={
                        "type": "object",
                        "properties": {
                            "to": {"type": "string"},
                            "subject": {"type": "string"},
                            "body": {"type": "string"},
                            "cc": {"type": "string"},
                            "html": {"type": "boolean", "default": False},
                        },
                        "required": ["to", "subject", "body"],
                    },
                ),
                Tool(
                    name="gmail_reply_to_message",
                    description="Reply to an email",
                    inputSchema={
                        "type": "object",
                        "properties": {
                            "message_id": {"type": "string"},
                            "body": {"type": "string"},
                            "html": {"type": "boolean", "default": False},
                        },
                        "required": ["message_id", "body"],
                    },
                ),

                # Gmail - Labels
                Tool(
                    name="gmail_list_labels",
                    description="List all Gmail labels",
                    inputSchema={"type": "object", "properties": {}},
                ),
                Tool(
                    name="gmail_create_label",
                    description="Create a new Gmail label",
                    inputSchema={
                        "type": "object",
                        "properties": {
                            "name": {"type": "string"},
                        },
                        "required": ["name"],
                    },
                ),
                Tool(
                    name="gmail_modify_message",
                    description="Modify labels on a message",
                    inputSchema={
                        "type": "object",
                        "properties": {
                            "message_id": {"type": "string"},
                            "add_labels": {"type": "array", "items": {"type": "string"}},
                            "remove_labels": {"type": "array", "items": {"type": "string"}},
                        },
                        "required": ["message_id"],
                    },
                ),

                # Gmail - Drafts
                Tool(
                    name="gmail_create_draft",
                    description="Create a draft email",
                    inputSchema={
                        "type": "object",
                        "properties": {
                            "to": {"type": "string"},
                            "subject": {"type": "string"},
                            "body": {"type": "string"},
                            "html": {"type": "boolean", "default": False},
                        },
                        "required": ["to", "subject", "body"],
                    },
                ),
                Tool(
                    name="gmail_send_draft",
                    description="Send a draft email",
                    inputSchema={
                        "type": "object",
                        "properties": {
                            "draft_id": {"type": "string"},
                        },
                        "required": ["draft_id"],
                    },
                ),

                # Calendar
                Tool(
                    name="calendar_list_events",
                    description="List calendar events",
                    inputSchema={
                        "type": "object",
                        "properties": {
                            "calendar_id": {"type": "string", "default": "primary"},
                            "max_results": {"type": "integer", "default": 100},
                            "query": {"type": "string"},
                        },
                    },
                ),
                Tool(
                    name="calendar_create_event",
                    description="Create a calendar event",
                    inputSchema={
                        "type": "object",
                        "properties": {
                            "summary": {"type": "string"},
                            "start_time": {"type": "string", "format": "date-time"},
                            "end_time": {"type": "string", "format": "date-time"},
                            "description": {"type": "string"},
                            "location": {"type": "string"},
                            "attendees": {"type": "array", "items": {"type": "string"}},
                        },
                        "required": ["summary", "start_time", "end_time"],
                    },
                ),
                Tool(
                    name="calendar_update_event",
                    description="Update a calendar event",
                    inputSchema={
                        "type": "object",
                        "properties": {
                            "event_id": {"type": "string"},
                            "calendar_id": {"type": "string", "default": "primary"},
                            "summary": {"type": "string"},
                            "start_time": {"type": "string", "format": "date-time"},
                            "end_time": {"type": "string", "format": "date-time"},
                        },
                        "required": ["event_id"],
                    },
                ),
                Tool(
                    name="calendar_delete_event",
                    description="Delete a calendar event",
                    inputSchema={
                        "type": "object",
                        "properties": {
                            "event_id": {"type": "string"},
                            "calendar_id": {"type": "string", "default": "primary"},
                        },
                        "required": ["event_id"],
                    },
                ),

                # People
                Tool(
                    name="people_list_contacts",
                    description="List contacts",
                    inputSchema={
                        "type": "object",
                        "properties": {
                            "page_size": {"type": "integer", "default": 100},
                        },
                    },
                ),
                Tool(
                    name="people_search_contacts",
                    description="Search contacts",
                    inputSchema={
                        "type": "object",
                        "properties": {
                            "query": {"type": "string"},
                        },
                        "required": ["query"],
                    },
                ),
                Tool(
                    name="people_create_contact",
                    description="Create a new contact",
                    inputSchema={
                        "type": "object",
                        "properties": {
                            "given_name": {"type": "string"},
                            "family_name": {"type": "string"},
                            "email": {"type": "string"},
                            "phone": {"type": "string"},
                        },
                        "required": ["given_name"],
                    },
                ),
                Tool(
                    name="people_delete_contact",
                    description="Delete a contact",
                    inputSchema={
                        "type": "object",
                        "properties": {
                            "resource_name": {"type": "string"},
                        },
                        "required": ["resource_name"],
                    },
                ),
            ]

        @self.server.call_tool()
        async def call_tool(name: str, arguments: Dict[str, Any]) -> List[TextContent]:
            """Execute a tool."""
            try:
                result = await self._execute_tool(name, arguments)
                return [TextContent(type="text", text=str(result))]
            except Exception as e:
                logger.error(f"Error executing tool {name}: {e}")
                return [TextContent(type="text", text=f"Error: {str(e)}")]

    async def _execute_tool(self, name: str, arguments: Dict[str, Any]) -> Any:
        """Execute a tool by name."""

        # Gmail tools
        if name == "gmail_list_messages":
            return self.gmail_service.list_messages(**arguments)

        elif name == "gmail_get_message":
            return self.gmail_service.get_message(**arguments)

        elif name == "gmail_send_message":
            return self.gmail_service.send_message(**arguments)

        elif name == "gmail_reply_to_message":
            return self.gmail_service.reply_to_message(**arguments)

        elif name == "gmail_list_labels":
            return self.gmail_service.list_labels()

        elif name == "gmail_create_label":
            return self.gmail_service.create_label(**arguments)

        elif name == "gmail_modify_message":
            return self.gmail_service.modify_message(**arguments)

        elif name == "gmail_create_draft":
            return self.gmail_service.create_draft(**arguments)

        elif name == "gmail_send_draft":
            return self.gmail_service.send_draft(**arguments)

        # Calendar tools
        elif name == "calendar_list_events":
            return self.calendar_service.list_events(**arguments)

        elif name == "calendar_create_event":
            from datetime import datetime
            arguments['start_time'] = datetime.fromisoformat(arguments['start_time'])
            arguments['end_time'] = datetime.fromisoformat(arguments['end_time'])
            return self.calendar_service.create_event(**arguments)

        elif name == "calendar_update_event":
            from datetime import datetime
            if 'start_time' in arguments:
                arguments['start_time'] = datetime.fromisoformat(arguments['start_time'])
            if 'end_time' in arguments:
                arguments['end_time'] = datetime.fromisoformat(arguments['end_time'])
            return self.calendar_service.update_event(**arguments)

        elif name == "calendar_delete_event":
            return self.calendar_service.delete_event(**arguments)

        # People tools
        elif name == "people_list_contacts":
            return self.people_service.list_contacts(**arguments)

        elif name == "people_search_contacts":
            return self.people_service.search_contacts(**arguments)

        elif name == "people_create_contact":
            return self.people_service.create_contact(**arguments)

        elif name == "people_delete_contact":
            return self.people_service.delete_contact(**arguments)

        else:
            raise ValueError(f"Unknown tool: {name}")

    def list_tools(self) -> List[Dict[str, Any]]:
        """List all available tools (for testing)."""
        # This is a synchronous wrapper for testing purposes
        # It returns the tool definitions as dicts
        tools = [
            {"name": "gmail_list_messages"},
            {"name": "gmail_get_message"},
            {"name": "gmail_send_message"},
            {"name": "gmail_reply_to_message"},
            {"name": "gmail_list_labels"},
            {"name": "gmail_create_label"},
            {"name": "gmail_modify_message"},
            {"name": "gmail_create_draft"},
            {"name": "gmail_send_draft"},
            {"name": "calendar_list_events"},
            {"name": "calendar_create_event"},
            {"name": "calendar_update_event"},
            {"name": "calendar_delete_event"},
            {"name": "people_list_contacts"},
            {"name": "people_search_contacts"},
            {"name": "people_create_contact"},
            {"name": "people_delete_contact"},
        ]
        return tools

    async def run(self) -> None:
        """Run the MCP server."""
        from mcp.server.stdio import stdio_server

        async with stdio_server() as (read_stream, write_stream):
            await self.server.run(
                read_stream,
                write_stream,
                self.server.create_initialization_options(),
            )


def main():
    """Main entry point."""
    import asyncio

    server = GSuiteMCPServer()
    asyncio.run(server.run())
