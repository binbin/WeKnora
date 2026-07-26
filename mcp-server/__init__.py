#!/usr/bin/env python3
"""
TreeRAG MCP Server Package

A Model Context Protocol server that provides access to the TreeRAG knowledge management API.
"""

__version__ = "1.0.0"
__author__ = "TreeRAG Team"
__description__ = "TreeRAG MCP Server - Model Context Protocol server for TreeRAG API"

from .weknora_mcp_server import TreeRAGClient, run

__all__ = ["TreeRAGClient", "run"]
