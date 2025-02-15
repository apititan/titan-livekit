import axios from "axios";

const pollingInterval = 2000;

export default () => {
    let intervalId;

    return  {
        methods: {
            invokeApiUserOnline(participantsProvider, handler) {
                const participants = participantsProvider();
                if (!participants || participants.length == 0) {
                    console.debug("Participants are empty or participantsProvider returned equal null, invoking handler with empty array");
                    handler([]);
                    return;
                }
                console.debug("Participants are non-empty, invoking axios");
                axios.get(`/api/user/online`, {
                    params: {
                        userId: participants.reduce((f, s) => `${f},${s}`)
                        // participantIds: [1,2,3].reduce((f, s) => `${f},${s}`)
                    }
                }).then(value => {
                    handler(value.data)
                })
            },
            startPolling(participantsProvider, handler) {
                this.invokeApiUserOnline(participantsProvider, handler);
                intervalId = setInterval(()=>{
                    this.invokeApiUserOnline(participantsProvider, handler);
                }, pollingInterval);
            },
            stopPolling() {
                if (intervalId) {
                    console.debug("Stopping user polling");
                    clearInterval(intervalId)
                }
            },
        },

    }
}
