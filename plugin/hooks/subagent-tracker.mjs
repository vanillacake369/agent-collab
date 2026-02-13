#!/usr/bin/env node

/**
 * Agent-Collab Subagent Tracker Hook
 * Tracks when sub-agents are spawned/stopped for multi-agent coordination
 *
 * Events:
 * - SubagentStart: A new sub-agent is being spawned
 * - SubagentStop: A sub-agent has completed or stopped
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

// Check if cluster is active
function isClusterActive() {
  try {
    const result = execSync(
      'echo \'{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"cluster_status","arguments":{}}}\' | agent-collab mcp serve 2>/dev/null',
      { encoding: 'utf-8', timeout: 2000 }
    );
    const parsed = JSON.parse(result);
    const status = JSON.parse(parsed.result?.content?.[0]?.text || '{}');
    return status.running === true;
  } catch {
    return false;
  }
}

// Share subagent event with cluster
function shareSubagentEvent(eventType, agentInfo) {
  try {
    const content = JSON.stringify({
      event: eventType,
      agent_type: agentInfo.subagent_type || 'unknown',
      description: agentInfo.description || '',
      model: agentInfo.model || 'inherit',
      background: agentInfo.run_in_background || false,
      timestamp: new Date().toISOString()
    });

    execSync(
      `echo '{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"share_context","arguments":{"file_path":"__subagent_event__","content":${JSON.stringify(content)}}}}' | agent-collab mcp serve 2>/dev/null`,
      { encoding: 'utf-8', timeout: 3000 }
    );
    return true;
  } catch {
    return false;
  }
}

// Main
async function main() {
  try {
    const input = await readStdin();
    let data = {};
    try { data = JSON.parse(input); } catch { /* ignore */ }

    // Determine event type from command line args or hook context
    const eventType = process.argv[2] || 'start'; // 'start' or 'stop'
    const toolInput = data.tool_input || data.toolInput || {};

    // Check if cluster is active
    if (!isClusterActive()) {
      console.log(JSON.stringify({ continue: true }));
      return;
    }

    if (eventType === 'start') {
      // SubagentStart: New agent being spawned
      const agentType = toolInput.subagent_type || 'unknown';
      const description = toolInput.description || '';

      shareSubagentEvent('subagent_start', toolInput);

      const message = `[SUBAGENT SPAWNED] Type: ${agentType}
Description: ${description}
Other agents in the cluster can see this activity.`;

      console.log(JSON.stringify({
        continue: true,
        hookSpecificOutput: {
          hookEventName: 'SubagentStart',
          additionalContext: message
        }
      }));

    } else if (eventType === 'stop') {
      // SubagentStop: Agent completed or stopped
      const result = data.result || data.tool_result || {};

      shareSubagentEvent('subagent_stop', {
        ...toolInput,
        result_summary: typeof result === 'string' ? result.slice(0, 200) : 'completed'
      });

      console.log(JSON.stringify({
        continue: true,
        hookSpecificOutput: {
          hookEventName: 'SubagentStop',
          additionalContext: '[SUBAGENT COMPLETED] Result shared with cluster.'
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
