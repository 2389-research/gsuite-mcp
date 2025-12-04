# ABOUTME: Tests for MCP server implementation
# ABOUTME: Validates server initialization and tool registration

import pytest
from unittest.mock import Mock, patch
from gsuite_mcp.server import GSuiteMCPServer


def test_server_initialization():
    """Server should initialize with all services."""
    with patch('gsuite_mcp.server.OAuth2Authenticator'):
        server = GSuiteMCPServer()

        assert server.name == "gsuite-mcp"
        assert hasattr(server, 'gmail_service')
        assert hasattr(server, 'calendar_service')
        assert hasattr(server, 'people_service')


def test_server_has_tools():
    """Server should register all MCP tools."""
    with patch('gsuite_mcp.server.OAuth2Authenticator'):
        server = GSuiteMCPServer()
        tools = server.list_tools()

        # Check for key tools from each service
        tool_names = [tool['name'] for tool in tools]

        assert 'gmail_list_messages' in tool_names
        assert 'gmail_send_message' in tool_names
        assert 'calendar_list_events' in tool_names
        assert 'calendar_create_event' in tool_names
        assert 'people_list_contacts' in tool_names
