export default class Logger {
    static logs = [];

    static log(msg) {
        Logger.logs.push(msg)

        //not working at all :(
        //const caller = arguments.callee.caller.name;
        //const callerParams = Function.caller.arguments;

        console.log(msg);

        //todo send to server
    }

    static getLast() {
        return logs[-1];
    }
}