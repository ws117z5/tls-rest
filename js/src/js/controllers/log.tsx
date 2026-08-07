export default class Logger {
    static logs: string[] = [];

    static log(...args: any[]) {
        // 1. Format all arguments (strings, objects, events, errors) cleanly
        const formattedMsg = args
        .map((arg) => {
            if (typeof arg === "string") return arg;
            if (arg instanceof Error) return arg.stack || arg.message;
            if (arg instanceof Event) return `[Event: ${arg.type}]`;
            try {
                return JSON.stringify(arg);
            } catch {
                return String(arg);
            }
        })
        .join(" ");

        // 2. Store formatted string in logs array
        Logger.logs.push(formattedMsg);

        // 3. Log original arguments to browser console (preserves interactive inspection)
        console.log(...args);

        // TODO: Send `formattedMsg` to server
    }

    /**
     * Helper to inspect caller location cleanly in modern JS/TS
     * (Replaces deprecated arguments.callee / Function.caller)
     */
    static getCallerLocation(): string {
        const err = new Error();
        if (!err.stack) return "unknown";

        const stackLines = err.stack.split("\n");
        // Line 0 is 'Error', Line 1 is 'getCallerLocation', Line 2 is 'log', Line 3 is the caller
        return stackLines[3] ? stackLines[3].trim() : "unknown";
    }

    static getLast() {
        return Logger.logs[Logger.logs.length - 1];
    }
}