#!/usr/bin/env node

/**
 * Agent-Collab Session End Hook
 * Automatically releases any locks held by this agent when session ends
 */

import { execSync } from 'child_process';
import { readFileSync, writeFileSync, existsSync } from 'fs';
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

// Check if cluster is active
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
      peerCount: status.peer_count || 0
    };
  } catch {
    return { active: false, peerCount: 0 };
  }
}

// Get locks from state file
function getSavedLocks() {
  try {
    if (existsSync(LOCK_STATE_FILE)) {
      return JSON.parse(readFileSync(LOCK_STATE_FILE, 'utf-8'));
    }
  } catch {
    // Ignore
  }
  return {};
}

// Clear lock state file
function clearLockState() {
  try {
    writeFileSync(LOCK_STATE_FILE, '{}');
  } catch {
    // Ignore
  }
}

// Release a lock via mcp call
function releaseLock(lockId) {
  try {
    const args = JSON.stringify({ lock_id: lockId });
    execSync(`agent-collab mcp call release_lock '${args}'`, {
      encoding: 'utf-8',
      timeout: 3000,
      stdio: ['pipe', 'pipe', 'pipe']
    });
    return true;
  } catch {
    return false;
  }
}

// Share session end event
function shareSessionEnd() {
  try {
    const args = JSON.stringify({
      file_path: '__session__',
      content: `Session ended at ${new Date().toISOString()}`
    });
    execSync(`agent-collab mcp call share_context '${args}'`, {
      encoding: 'utf-8',
      timeout: 3000,
      stdio: ['pipe', 'pipe', 'pipe']
    });
  } catch {
    // Best effort
  }
}

// Main
async function main() {
  try {
    await readStdin();

    // Check if cluster is active
    const cluster = getClusterStatus();
    if (!cluster.active) {
      console.log(JSON.stringify({ continue: true }));
      return;
    }

    // Get and release all locks from state file
    const savedLocks = getSavedLocks();
    let releasedCount = 0;

    for (const [filePath, lockInfo] of Object.entries(savedLocks)) {
      if (lockInfo.lockId && releaseLock(lockInfo.lockId)) {
        releasedCount++;
      }
    }

    // Clear lock state
    clearLockState();

    // Notify cluster of session end
    shareSessionEnd();

    if (releasedCount > 0) {
      console.log(JSON.stringify({
        continue: true,
        hookSpecificOutput: {
          hookEventName: 'SessionEnd',
          additionalContext: `[SESSION END] Released ${releasedCount} lock(s) → ${cluster.peerCount} peer(s) notified`
        }
      }));
    } else {
      console.log(JSON.stringify({
        continue: true,
        hookSpecificOutput: {
          hookEventName: 'SessionEnd',
          additionalContext: `[SESSION END] No locks to release. ${cluster.peerCount} peer(s) notified`
        }
      }));
    }

  } catch {
    console.log(JSON.stringify({ continue: true }));
  }
}

main();
