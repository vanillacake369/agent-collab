/**
 * Shared constants for agent-collab hooks
 */

import { join } from 'path';
import os from 'os';

// Timeout constants (in milliseconds)
export const TIMEOUTS = {
    CLUSTER_STATUS: 3000,  // 3 seconds
    LOCK_ACQUIRE: 5000,    // 5 seconds
    LOCK_RELEASE: 3000,    // 3 seconds
    CONTEXT_SHARE: 5000,   // 5 seconds
    STDIN_READ: 5000,      // 5 seconds
    DEFAULT: 3000,         // 3 seconds
};

// Path constants
export const PATHS = {
    /**
     * Get the data directory for agent-collab
     * @returns {string} Path to data directory
     */
    getDataDir: () => {
        if (process.env.AGENT_COLLAB_DATA_DIR) {
            return process.env.AGENT_COLLAB_DATA_DIR;
        }
        const homeDir = process.env.HOME || process.env.USERPROFILE || os.homedir();
        return join(homeDir, '.agent-collab');
    },
    LOCK_STATE_FILE: 'hook-locks.json',
};

// Lock constants
export const LOCK = {
    START_LINE_DEFAULT: 1,
    END_LINE_ENTIRE_FILE: -1,
};

// Detection constants (for collab-detector)
export const DETECTION = {
    MIN_PROMPT_LENGTH: 5,
    MAX_SANITIZED_LENGTH: 10000,
    MAX_SIMILAR_CONTEXTS: 10,
    MAX_RECENT_ACTIVITY: 20,
};

// MCP tool names
export const MCP_TOOLS = {
    CLUSTER_STATUS: 'cluster_status',
    ACQUIRE_LOCK: 'acquire_lock',
    RELEASE_LOCK: 'release_lock',
    SHARE_CONTEXT: 'share_context',
    GET_EVENTS: 'get_events',
    GET_WARNINGS: 'get_warnings',
    SEARCH_SIMILAR: 'search_similar',
};

// Hook output keys
export const HOOK_OUTPUT = {
    CONTINUE: 'continue',
    HOOK_SPECIFIC_OUTPUT: 'hookSpecificOutput',
    HOOK_EVENT_NAME: 'hookEventName',
    ADDITIONAL_CONTEXT: 'additionalContext',
};

// Message prefixes for hook output
export const MESSAGE_PREFIXES = {
    AUTO_LOCK: '[AUTO-LOCK]',
    AUTO_SHARED: '[AUTO-SHARED]',
    LOCK_CONFLICT: '[LOCK CONFLICT]',
    LOCK_REUSED: '[LOCK REUSED]',
    SESSION_END: '[SESSION END]',
    CLUSTER_ACTIVE: '[AGENT-COLLAB CLUSTER ACTIVE]',
};
