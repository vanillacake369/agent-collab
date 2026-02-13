#!/usr/bin/env node

/**
 * Agent-Collab PostToolUse Hook
 * Automatically shares context after Edit/Write operations when cluster is active
 */

import { execSync } from 'child_process';

// Read stdin
async function readStdin() {
  const chunks = [];
  for await (const chunk of process.stdin) {
    chunks.push(chunk);
  }
  return Buffer.concat(chunks).toString('utf-8');
}

// Check if daemon is running and cluster has peers
function getClusterStatus() {
  try {
    const result = execSync('agent-collab mcp call cluster_status \'{}\'', {
      encoding: 'utf-8',
      timeout: 3000,
      stdio: ['pipe', 'pipe', 'pipe']
    });
    const status = JSON.parse(result);
    return {
      active: status.running === true && (status.peer_count || 0) > 0,
      peerCount: status.peer_count || 0,
      projectName: status.project_name || ''
    };
  } catch {
    return { active: false, peerCount: 0, projectName: '' };
  }
}

// Automatically share context via mcp call
function shareContext(filePath, content) {
  try {
    const args = JSON.stringify({
      file_path: filePath,
      content: content
    });
    const result = execSync(`agent-collab mcp call share_context '${args}'`, {
      encoding: 'utf-8',
      timeout: 5000,
      stdio: ['pipe', 'pipe', 'pipe']
    });
    const parsed = JSON.parse(result);
    return {
      success: parsed.success === true,
      documentId: parsed.document_id || '',
      message: parsed.message || ''
    };
  } catch (err) {
    return { success: false, error: err.message };
  }
}

// Extract change summary from tool input/result
function extractChangeSummary(toolName, toolInput, toolResult) {
  const filePath = toolInput.file_path || toolInput.filePath || 'unknown';

  if (toolName === 'Edit') {
    const oldStr = toolInput.old_string || '';
    const newStr = toolInput.new_string || '';
    const oldPreview = oldStr.length > 50 ? oldStr.substring(0, 50) + '...' : oldStr;
    const newPreview = newStr.length > 50 ? newStr.substring(0, 50) + '...' : newStr;
    return `Edit: replaced "${oldPreview}" with "${newPreview}"`;
  }

  if (toolName === 'Write') {
    const content = toolInput.content || '';
    const lines = content.split('\n').length;
    const preview = content.substring(0, 100).replace(/\n/g, ' ');
    return `Write: created/updated file (${lines} lines) - ${preview}...`;
  }

  return `${toolName} operation on ${filePath}`;
}

// Main
async function main() {
  try {
    const input = await readStdin();
    let data = {};
    try { data = JSON.parse(input); } catch { /* ignore */ }

    const toolName = data.tool_name || data.toolName || '';
    const toolInput = data.tool_input || data.toolInput || {};
    const toolResult = data.tool_result || data.result || {};

    // Only handle Edit/Write
    if (!['Edit', 'Write'].includes(toolName)) {
      console.log(JSON.stringify({ continue: true }));
      return;
    }

    // Check if tool succeeded
    const success = !toolResult.error && !toolResult.isError;
    if (!success) {
      console.log(JSON.stringify({ continue: true }));
      return;
    }

    // Check if cluster is active (has peers)
    const cluster = getClusterStatus();
    if (!cluster.active) {
      // No cluster or no peers - skip auto-sharing
      console.log(JSON.stringify({ continue: true }));
      return;
    }

    // Cluster is active - automatically share context
    const filePath = toolInput.file_path || toolInput.filePath || 'file';
    const changeSummary = extractChangeSummary(toolName, toolInput, toolResult);

    const shareResult = shareContext(filePath, changeSummary);

    let message;
    if (shareResult.success) {
      message = `[AUTO-SHARED] ${filePath} → ${cluster.peerCount} peer(s) (${shareResult.documentId})`;
    } else {
      message = `[SHARE FAILED] ${filePath}: ${shareResult.error || 'unknown error'}`;
    }

    console.log(JSON.stringify({
      continue: true,
      hookSpecificOutput: {
        hookEventName: 'PostToolUse',
        additionalContext: message
      }
    }));
  } catch (err) {
    // Don't block on errors
    console.log(JSON.stringify({ continue: true }));
  }
}

main();
