#!/usr/bin/env node

/**
 * Agent-Collab Session Start Hook
 * Checks cluster status and injects collaboration reminders on session start
 */

import { execSync } from 'child_process';

// Read stdin with timeout
async function readStdin(timeoutMs = 5000) {
  return new Promise((resolve) => {
    const chunks = [];
    let settled = false;

    const timeout = setTimeout(() => {
      if (!settled) {
        settled = true;
        process.stdin.removeAllListeners();
        resolve(Buffer.concat(chunks).toString('utf-8'));
      }
    }, timeoutMs);

    process.stdin.on('data', (chunk) => chunks.push(chunk));
    process.stdin.on('end', () => {
      if (!settled) {
        settled = true;
        clearTimeout(timeout);
        resolve(Buffer.concat(chunks).toString('utf-8'));
      }
    });
    process.stdin.on('error', () => {
      if (!settled) {
        settled = true;
        clearTimeout(timeout);
        resolve('');
      }
    });
  });
}

// Check cluster status
function getClusterStatus() {
  try {
    const result = execSync(
      'echo \'{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"cluster_status","arguments":{}}}\' | agent-collab mcp serve 2>/dev/null',
      { encoding: 'utf-8', timeout: 3000 }
    );
    const parsed = JSON.parse(result);
    return JSON.parse(parsed.result?.content?.[0]?.text || '{}');
  } catch {
    return { running: false, peer_count: 0 };
  }
}

// Get recent events
function getRecentEvents(limit = 5) {
  try {
    const result = execSync(
      `echo '{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"get_events","arguments":{"limit":${limit}}}}' | agent-collab mcp serve 2>/dev/null`,
      { encoding: 'utf-8', timeout: 3000 }
    );
    const parsed = JSON.parse(result);
    return JSON.parse(parsed.result?.content?.[0]?.text || '[]');
  } catch {
    return [];
  }
}

// Get warnings
function getWarnings() {
  try {
    const result = execSync(
      'echo \'{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"get_warnings","arguments":{}}}\' | agent-collab mcp serve 2>/dev/null',
      { encoding: 'utf-8', timeout: 3000 }
    );
    const parsed = JSON.parse(result);
    return JSON.parse(parsed.result?.content?.[0]?.text || '[]');
  } catch {
    return [];
  }
}

// Main
async function main() {
  try {
    await readStdin();
    const messages = [];

    const status = getClusterStatus();

    if (status.running && status.peer_count > 0) {
      // Cluster is active - provide collaboration context
      const events = getRecentEvents(5);
      const warnings = getWarnings();

      let clusterInfo = `[AGENT-COLLAB CLUSTER CONNECTED]

Peer count: ${status.peer_count}
Node ID: ${status.node_id || 'unknown'}
`;

      if (warnings.length > 0) {
        clusterInfo += `\nActive Warnings: ${warnings.length}\n`;
        warnings.forEach(w => {
          clusterInfo += `- ${w.message || w}\n`;
        });
      }

      if (events.length > 0) {
        clusterInfo += `\nRecent Activity:\n`;
        events.slice(0, 3).forEach(e => {
          const eventType = e.type || e.event_type || 'unknown';
          const peer = e.peer_id?.slice(0, 8) || 'unknown';
          clusterInfo += `- [${eventType}] from peer ${peer}\n`;
        });
      }

      clusterInfo += `
Collaboration Protocol Active:
- Use search_similar before starting work
- Use acquire_lock before modifying files
- Use share_context after completing work
- Use release_lock when done
`;

      messages.push(`<session-start>\n${clusterInfo}\n</session-start>`);

    } else if (status.running) {
      // Daemon running but no peers
      messages.push(`<session-start>
[AGENT-COLLAB] Daemon running but no peers connected.
Other agents can join the cluster for collaborative work.
</session-start>`);
    } else {
      // Daemon not running
      messages.push(`<session-start>
[AGENT-COLLAB] Cluster not active.
To enable multi-agent collaboration, run: agent-collab daemon start
</session-start>`);
    }

    if (messages.length > 0) {
      console.log(JSON.stringify({
        continue: true,
        hookSpecificOutput: {
          hookEventName: 'SessionStart',
          additionalContext: messages.join('\n')
        }
      }));
    } else {
      console.log(JSON.stringify({ continue: true }));
    }
  } catch {
    console.log(JSON.stringify({ continue: true }));
  }
}

main();
