// This is your plugin object. It can be exported to be used anywhere.
import { Centrifuge } from "centrifuge";
import {getWebsocketUrlPrefix} from "./utils";

export const setupCentrifuge = (centrifugeSessionFunction, onDisconnected) => {
    // Create Centrifuge object with Websocket endpoint address set in main.go
    var url = getWebsocketUrlPrefix() + "/api/chat/websocket";
    var centrifuge = new Centrifuge(url, {
        protocol: 'json',
        debug: true,
        minReconnectDelay: 1000,
        maxReconnectDelay: 1000,
        timeout: 4000,
        pingInterval: 5000
    });
    centrifuge.on('connected', (ctx)=>{
        console.log("Connected response", ctx);
        centrifugeSessionFunction(ctx.client);
        console.log('My centrifuge session clientId :', ctx.client);
    });
    centrifuge.on('disconnected', (ctx)=>{
        console.log("Disconnected response", ctx);
        onDisconnected();
    });

    return centrifuge;
};

export const getData = (message) => {
    return message.data
};

export const getProperData = (message) => {
    return message.data.payload
};

export const setProperData = (message) => {
    return {
        payload: message
    }
};
