#!/usr/bin/env node

/**
 * Agent-Collab PreToolUse Hook
 * Automatically acquires locks before Edit/Write operations when cluster is active
 */

import { execSync } from 'child_process';
import { writeFileSync, readFileSync, existsSync, mkdirSync } from 'fs';
import { join } from 'path';

// Lock state file for tracking acquired locks
const LOCK_STATE_DIR = process.env.AGENT_COLLAB_DATA_DIR || join(process.env.HOME || '/tmp', '.agent-collab');
const LOCK_STATE_FILE = join(LOCK_STATE_DIR, 'hook-locks.json');

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

// Acquire lock via mcp call
function acquireLock(filePath, startLine, endLine, intention) {
  try {
    const args = JSON.stringify({
      file_path: filePath,
      start_line: startLine,
      end_line: endLine,
      intention: intention
    });
    const result = execSync(`agent-collab mcp call acquire_lock '${args}'`, {
      encoding: 'utf-8',
      timeout: 5000,
      stdio: ['pipe', 'pipe', 'pipe']
    });
    const parsed = JSON.parse(result);
    return {
      success: parsed.success === true,
      lockId: parsed.lock_id || '',
      error: parsed.error || ''
    };
  } catch (err) {
    return { success: false, error: err.message };
  }
}

// Save lock to state file for later release
function saveLockState(filePath, lockId) {
  try {
    if (!existsSync(LOCK_STATE_DIR)) {
      mkdirSync(LOCK_STATE_DIR, { recursive: true });
    }
    let state = {};
    if (existsSync(LOCK_STATE_FILE)) {
      state = JSON.parse(readFileSync(LOCK_STATE_FILE, 'utf-8'));
    }
    state[filePath] = {
      lockId: lockId,
      acquiredAt: new Date().toISOString()
    };
    writeFileSync(LOCK_STATE_FILE, JSON.stringify(state, null, 2));
  } catch {
    // Ignore state save errors
  }
}

// Check if we already have a lock for this file
function getExistingLock(filePath) {
  try {
    if (existsSync(LOCK_STATE_FILE)) {
      const state = JSON.parse(readFileSync(LOCK_STATE_FILE, 'utf-8'));
      return state[filePath] || null;
    }
  } catch {
    // Ignore
  }
  return null;
}

// Main
async function main() {
  try {
    const input = await readStdin();
    let data = {};
    try { data = JSON.parse(input); } catch { /* ignore */ }

    const toolName = data.tool_name || data.toolName || '';
    const toolInput = data.tool_input || data.toolInput || {};

    // Only handle Edit/Write
    if (!['Edit', 'Write'].includes(toolName)) {
      console.log(JSON.stringify({ continue: true }));
      return;
    }

    // Check if cluster is active (has peers)
    const cluster = getClusterStatus();
    if (!cluster.active) {
      // No cluster or no peers - skip lock acquisition
      console.log(JSON.stringify({ continue: true }));
      return;
    }

    // Get file path from tool input
    const filePath = toolInput.file_path || toolInput.filePath || '';
    if (!filePath) {
      console.log(JSON.stringify({ continue: true }));
      return;
    }

    // Check if we already have a lock for this file
    const existingLock = getExistingLock(filePath);
    if (existingLock) {
      console.log(JSON.stringify({
        continue: true,
        hookSpecificOutput: {
          hookEventName: 'PreToolUse',
          additionalContext: `[LOCK REUSED] ${filePath} (${existingLock.lockId})`
        }
      }));
      return;
    }

    // Determine line range
    const startLine = toolInput.start_line || 1;
    const endLine = toolInput.end_line || -1; // -1 means entire file
    const intention = `${toolName} operation on ${filePath}`;

    // Acquire lock
    const lockResult = acquireLock(filePath, startLine, endLine, intention);

    let message;
    if (lockResult.success) {
      // Save lock for later release
      saveLockState(filePath, lockResult.lockId);
      message = `[AUTO-LOCK] ${filePath} acquired (${lockResult.lockId})`;
    } else {
      // Lock failed - warn but don't block
      message = `[LOCK CONFLICT] ${filePath}: ${lockResult.error || 'another agent may be working on this file'}`;
    }

    console.log(JSON.stringify({
      continue: true,
      hookSpecificOutput: {
        hookEventName: 'PreToolUse',
        additionalContext: message
      }
    }));
  } catch (err) {
    // Don't block on errors
    console.log(JSON.stringify({ continue: true }));
  }
}

main();
