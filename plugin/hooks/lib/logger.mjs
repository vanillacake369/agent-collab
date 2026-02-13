/**
 * Simple structured logger for agent-collab hooks
 *
 * Outputs to stderr since Claude Code reads stdout for hook responses.
 */

/**
 * Create a logger for a specific component
 * @param {string} component - Component name (e.g., 'hook:pre-tool-enforcer')
 * @returns {Object} Logger object with debug, info, warn, error methods
 */
export function createLogger(component) {
    const log = (level, msg, meta = {}) => {
        const entry = {
            level,
            component,
            message: msg,
            ...meta,
            timestamp: new Date().toISOString(),
        };
        // Output to stderr (stdout is reserved for hook JSON response)
        console.error(JSON.stringify(entry));
    };

    return {
        /**
         * Log at debug level
         * @param {string} msg - Message to log
         * @param {Object} meta - Additional metadata
         */
        debug: (msg, meta) => log('DEBUG', msg, meta),

        /**
         * Log at info level
         * @param {string} msg - Message to log
         * @param {Object} meta - Additional metadata
         */
        info: (msg, meta) => log('INFO', msg, meta),

        /**
         * Log at warn level
         * @param {string} msg - Message to log
         * @param {Object} meta - Additional metadata
         */
        warn: (msg, meta) => log('WARN', msg, meta),

        /**
         * Log at error level
         * @param {string} msg - Message to log
         * @param {Object} meta - Additional metadata
         */
        error: (msg, meta) => log('ERROR', msg, meta),
    };
}

/**
 * Get the log level from environment or default to 'info'
 * @returns {string} Log level
 */
export function getLogLevel() {
    return process.env.AGENT_COLLAB_LOG_LEVEL || 'info';
}

/**
 * Check if a log level should be logged
 * @param {string} level - Level to check
 * @returns {boolean} Whether to log
 */
export function shouldLog(level) {
    const levels = ['debug', 'info', 'warn', 'error'];
    const currentLevel = getLogLevel().toLowerCase();
    const currentIndex = levels.indexOf(currentLevel);
    const levelIndex = levels.indexOf(level.toLowerCase());
    return levelIndex >= currentIndex;
}
