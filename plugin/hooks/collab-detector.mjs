#!/usr/bin/env node

/**
 * Agent-Collab Collaboration Detector Hook (Node.js)
 * Detects collaboration-related intents and injects MCP tool usage instructions
 *
 * This runs on UserPromptSubmit and checks if:
 * 1. Cluster is active (daemon running with peers)
 * 2. User's prompt suggests collaborative work
 *
 * If both conditions are met, it injects context telling Claude to use MCP tools:
 * - search_similar: Before starting work
 * - acquire_lock: Before modifying files
 * - share_context: After completing work
 * - release_lock: After sharing context
 */

import { execSync } from 'child_process';

// Read all stdin
async function readStdin() {
  const chunks = [];
  for await (const chunk of process.stdin) {
    chunks.push(chunk);
  }
  return Buffer.concat(chunks).toString('utf-8');
}

// Extract prompt from various JSON structures
function extractPrompt(input) {
  try {
    const data = JSON.parse(input);
    if (data.prompt) return data.prompt;
    if (data.message?.content) return data.message.content;
    if (Array.isArray(data.parts)) {
      return data.parts
        .filter(p => p.type === 'text')
        .map(p => p.text)
        .join(' ');
    }
    return '';
  } catch {
    const match = input.match(/"(?:prompt|content|text)"\s*:\s*"([^"]+)"/);
    return match ? match[1] : '';
  }
}

// Sanitize text to prevent false positives from code blocks, URLs, file paths
function sanitizeForDetection(text) {
  return text
    .replace(/<(\w[\w-]*)[\s>][\s\S]*?<\/\1>/g, '')  // XML tags
    .replace(/<\w[\w-]*(?:\s[^>]*)?\s*\/>/g, '')      // Self-closing tags
    .replace(/https?:\/\/[^\s)>\]]+/g, '')            // URLs
    .replace(/(?:\/)?(?:[\w.-]+\/)+[\w.-]+/gm, '')    // File paths
    .replace(/```[\s\S]*?```/g, '')                   // Code blocks
    .replace(/`[^`]+`/g, '');                         // Inline code
}

// Check if agent-collab cluster is active
function isClusterActive() {
  try {
    const result = execSync(
      'echo \'{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"cluster_status","arguments":{}}}\' | agent-collab mcp serve 2>/dev/null',
      { encoding: 'utf-8', timeout: 3000 }
    );
    const parsed = JSON.parse(result);
    const status = JSON.parse(parsed.result?.content?.[0]?.text || '{}');
    return status.running === true && (status.peer_count || 0) > 0;
  } catch {
    return false;
  }
}

// Detect collaboration-related keywords/intents
function detectCollabIntent(prompt) {
  const cleanPrompt = sanitizeForDetection(prompt).toLowerCase();

  // Explicit collaboration keywords (highest priority)
  const explicitCollabKeywords = [
    // English
    /\b(collaborate|collaboration|teamwork|together|coordinate|coordination)\b/i,
    /\b(other\s*agent|another\s*agent|team\s*member|peer)\b/i,
    /\b(share\s*(this|my|the|context|work)|broadcast|notify\s*team)\b/i,
    /\b(check\s*(cluster|team|status|lock)|who\s*is\s*working)\b/i,
    /\b(conflict|collision|overlap|duplicate\s*work)\b/i,
    // Korean
    /\b(협업|팀워크|함께|같이|다같이|분담|나눠서)\b/i,
    /\b(다른\s*(에이전트|개발자)|팀원|동료)\b/i,
    /\b(공유|전달|알려|브로드캐스트)\b/i,
    /\b(클러스터|상태\s*확인|락\s*확인|누가\s*작업)\b/i,
    /\b(충돌|겹침|중복\s*작업)\b/i,
  ];

  // Work-related keywords that benefit from collaboration
  const workKeywords = [
    // File modification intents - English
    /\b(implement|create|add|modify|update|change|fix|refactor|delete|remove)\b/i,
    /\b(write|edit|develop|build|make)\b/i,
    // File modification intents - Korean
    /\b(구현|작성|추가|수정|변경|고치|리팩터|삭제|제거)\b/i,
    /\b(개발|만들|빌드)\b/i,
    // Code-related actions
    /\b(function|method|class|interface|component|module|service|handler|controller|api)\b/i,
    /\b(함수|메서드|클래스|인터페이스|컴포넌트|모듈|서비스|핸들러|컨트롤러)\b/i,
    // File types
    /\.(go|ts|tsx|js|jsx|py|java|rs|rb|cpp|c|h|swift|kt|scala)\b/i,
    /\b(main|config|handler|service|model|controller|util|helper|test)\./i,
  ];

  // Task completion keywords
  const completionKeywords = [
    /\b(done|finished|completed|완료|끝|다\s*했)/i,
    /\b(share\s*(this|what|my)|공유해|알려줘)\b/i,
  ];

  // Before-work keywords
  const beforeWorkKeywords = [
    /\b(before\s*(I|we)\s*(start|begin)|작업\s*전|시작\s*전)\b/i,
    /\b(check\s*(first|before)|먼저\s*확인|사전\s*확인)\b/i,
    /\b(what\s*(is|are)\s*(other|team)|다른\s*(사람|에이전트)\s*(뭐|무엇))\b/i,
  ];

  const hasExplicitCollab = explicitCollabKeywords.some(kw => kw.test(cleanPrompt));
  const hasWorkKeyword = workKeywords.some(kw => kw.test(cleanPrompt));
  const hasCompletion = completionKeywords.some(kw => kw.test(cleanPrompt));
  const hasBeforeWork = beforeWorkKeywords.some(kw => kw.test(cleanPrompt));

  return {
    isExplicitCollab: hasExplicitCollab,
    isWork: hasWorkKeyword,
    isCompletion: hasCompletion,
    isBeforeWork: hasBeforeWork,
    // Inject if: explicit collab OR (work keyword AND cluster active)
    shouldInject: hasExplicitCollab || hasWorkKeyword || hasCompletion || hasBeforeWork
  };
}

// Generate collaboration context injection
function generateCollabContext(intent) {
  // Customize message based on intent type
  let contextMessage = '';

  if (intent.isBeforeWork) {
    contextMessage = `
## Recommended: Check Before Starting
Since you mentioned checking before work, call these MCP tools:
1. \`search_similar\` - Find related work by other agents
2. \`get_warnings\` - Check for any conflicts or alerts
3. \`check_cohesion\` - Verify your plan aligns with team direction
`;
  } else if (intent.isCompletion) {
    contextMessage = `
## Recommended: Share Your Completed Work
Since you finished work, call these MCP tools:
1. \`share_context\` - Share what you did with the team
2. \`release_lock\` - Release any locks you hold
3. \`check_cohesion\` - Verify your changes align with team direction
`;
  }

  const collabInstructions = `<agent-collab-context>

[AGENT-COLLAB CLUSTER ACTIVE]

You are connected to an agent-collab cluster with other AI agents working in parallel.
${contextMessage}
## Collaboration Protocol for File Modifications

### Before Starting Work
1. Call \`search_similar\` with relevant keywords to check what other agents have done
2. Call \`get_warnings\` to check for any active alerts

### Before Modifying Files
3. Call \`acquire_lock\` with:
   - file_path: the file you want to modify
   - start_line, end_line: the line range you'll modify
   - intention: brief description of your changes
4. If lock fails, work on a different task or wait

### After Completing Work
5. Call \`share_context\` with:
   - file_path: the file you modified
   - content: summary of changes, reasons, and any interfaces other agents should know
6. Call \`release_lock\` with the lock_id from step 3

## Available MCP Tools
- \`cluster_status\` - Check cluster connection
- \`search_similar\` - Find related context
- \`get_warnings\` - Get active alerts
- \`acquire_lock\` - Lock a file before editing
- \`release_lock\` - Release a lock
- \`share_context\` - Share your work
- \`check_cohesion\` - Verify alignment with team

</agent-collab-context>

---
`;

  return collabInstructions;
}

// Create hook output with additional context
function createHookOutput(additionalContext) {
  return {
    continue: true,
    hookSpecificOutput: {
      hookEventName: 'UserPromptSubmit',
      additionalContext
    }
  };
}

// Main
async function main() {
  try {
    const input = await readStdin();
    if (!input.trim()) {
      console.log(JSON.stringify({ continue: true }));
      return;
    }

    const prompt = extractPrompt(input);
    if (!prompt) {
      console.log(JSON.stringify({ continue: true }));
      return;
    }

    // Check if cluster is active
    const clusterActive = isClusterActive();
    if (!clusterActive) {
      // No cluster, no collaboration needed
      console.log(JSON.stringify({ continue: true }));
      return;
    }

    // Detect collaboration intent
    const intent = detectCollabIntent(prompt);
    if (!intent.shouldInject) {
      // No relevant keywords, skip injection
      console.log(JSON.stringify({ continue: true }));
      return;
    }

    // Inject collaboration context
    const context = generateCollabContext(intent);
    console.log(JSON.stringify(createHookOutput(context)));

  } catch {
    // On error, always continue without blocking
    console.log(JSON.stringify({ continue: true }));
  }
}

main();
