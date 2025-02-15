<template>
    <v-col cols="12" class="ma-0 pa-0" id="video-container">
    </v-col>
</template>

<script>
import Vue from 'vue';
import {mapGetters} from 'vuex';
import {
    Room,
    RoomEvent,
    VideoPresets,
    createLocalTracks,
    createLocalScreenTracks,
} from 'livekit-client';
import UserVideo from "./UserVideo";
import vuetify from "@/plugins/vuetify";
import { v4 as uuidv4 } from 'uuid';
import axios from "axios";
import { retry } from '@lifeomic/attempt';
import {
    SET_SHOW_CALL_BUTTON,
    SET_SHOW_HANG_BUTTON,
    SET_SHOW_RECORD_START_BUTTON,
    GET_SHOW_RECORD_STOP_BUTTON,
    SET_VIDEO_CHAT_USERS_COUNT, SET_SHOW_RECORD_STOP_BUTTON, GET_CAN_MAKE_RECORD
} from "@/store";
import {
    defaultAudioMute,
    getStoredAudioDevicePresents,
    getStoredVideoDevicePresents,
    getWebsocketUrlPrefix,
    isMobileFireFox
} from "@/utils";
import bus, {
    ADD_SCREEN_SOURCE,
    ADD_VIDEO_SOURCE,
    REQUEST_CHANGE_VIDEO_PARAMETERS,
    VIDEO_PARAMETERS_CHANGED
} from "@/bus";
import {ChatVideoUserComponentHolder} from "@/ChatVideoUserComponentHolder";
import {chat_name, videochat_name} from "@/routes";
import videoServerSettingsMixin from "@/videoServerSettingsMixin";
import queryMixin from "@/queryMixin";

const UserVideoClass = Vue.extend(UserVideo);

const first = 'first';
const second = 'second';
const last = 'last';

export default {
    mixins: [videoServerSettingsMixin(), queryMixin()],

    data() {
        return {
            chatId: null,
            room: null,
            videoContainerDiv: null,
            userVideoComponents: new ChatVideoUserComponentHolder(),
            inRestarting: false,
        }
    },
    methods: {
        getNewId() {
            return uuidv4();
        },

        createComponent(userIdentity, position, videoTagId, localVideoProperties) {
            const component = new UserVideoClass({vuetify: vuetify,
                propsData: {
                    id: videoTagId,
                    localVideoProperties: localVideoProperties
                }
            });
            component.$mount();
            if (position == first) {
                this.insertChildAtIndex(this.videoContainerDiv, component.$el, 0);
            } else if (position == last) {
                this.videoContainerDiv.append(component.$el);
            } else if (position == second) {
                this.insertChildAtIndex(this.videoContainerDiv, component.$el, 1);
            }
            this.userVideoComponents.addComponentForUser(userIdentity, component);
            return component;
        },
        insertChildAtIndex(element, child, index) {
            if (!index) index = 0
            if (index >= element.children.length) {
                element.appendChild(child)
            } else {
                element.insertBefore(child, element.children[index])
            }
        },
        videoPublicationIsPresent (videoStream, userVideoComponents) {
            return !!userVideoComponents.filter(e => e.getVideoStreamId() == videoStream.trackSid).length
        },
        audioPublicationIsPresent (audioStream, userVideoComponents) {
            return !!userVideoComponents.filter(e => e.getAudioStreamId() == audioStream.trackSid).length
        },
        drawNewComponentOrInsertIntoExisting(participant, participantTrackPublications, position, localVideoProperties) {
            const md = JSON.parse((participant.metadata));
            const prefix = localVideoProperties ? 'local-' : 'remote-';
            const videoTagId = prefix + this.getNewId();

            const participantIdentityString = participant.identity;
            const components = this.userVideoComponents.getByUser(participantIdentityString);
            const candidatesWithoutVideo = components.filter(e => !e.getVideoStreamId());
            const candidatesWithoutAudio = components.filter(e => !e.getAudioStreamId());

            for (const track of participantTrackPublications) {
                if (track.kind == 'video') {
                    console.debug("Processing video track", track);
                    if (this.videoPublicationIsPresent(track, components)) {
                        console.debug("Skipping video", track);
                        continue;
                    }
                    let candidateToAppendVideo = candidatesWithoutVideo.length ? candidatesWithoutVideo[0] : null;
                    console.debug("candidatesWithoutVideo", candidatesWithoutVideo, "candidateToAppendVideo", candidateToAppendVideo);
                    if (!candidateToAppendVideo) {
                        candidateToAppendVideo = this.createComponent(participantIdentityString, position, videoTagId, localVideoProperties);
                    }
                    //const cameraEnabled = track && track.isSubscribed && !track.isMuted;
                    const cameraEnabled = track && !track.isMuted;// && track.isSubscribed && !track.isMuted;
                    if (!track.isSubscribed) {
                        console.warn("Video track is not subscribed");
                    }
                    candidateToAppendVideo.setVideoStream(track, cameraEnabled);
                    console.log("Video track was set", track.trackSid, "to", candidateToAppendVideo.getId());
                    candidateToAppendVideo.setUserName(md.login);
                    candidateToAppendVideo.setAvatar(md.avatar);
                    candidateToAppendVideo.setUserId(md.userId);
                    return
                } else if (track.kind == 'audio') {
                    console.debug("Processing audio track", track);
                    if (this.audioPublicationIsPresent(track, components)) {
                        console.debug("Skipping audio", track);
                        continue;
                    }
                    let candidateToAppendAudio = candidatesWithoutAudio.length ? candidatesWithoutAudio[0] : null;
                    console.debug("candidatesWithoutAudio", candidatesWithoutAudio, "candidateToAppendAudio", candidateToAppendAudio);
                    if (!candidateToAppendAudio) {
                        candidateToAppendAudio = this.createComponent(participantIdentityString, position, videoTagId, localVideoProperties);
                    }
                    //const micEnabled = track && track.isSubscribed && !track.isMuted;
                    const micEnabled = track && !track.isMuted// && track.isSubscribed && !track.isMuted;
                    if (!track.isSubscribed) {
                        console.warn("Audio track is not subscribed");
                    }
                    candidateToAppendAudio.setAudioStream(track, micEnabled);
                    console.log("Audio track was set", track.trackSid, "to", candidateToAppendAudio.getId());
                    candidateToAppendAudio.setUserName(md.login);
                    candidateToAppendAudio.setAvatar(md.avatar);
                    candidateToAppendAudio.setUserId(md.userId);
                    return
                }
            }
            this.setError(null, "Unable to draw track", participantTrackPublications);
            return
        },

        handleTrackUnsubscribed(
            track,
            publication,
            participant,
        ) {
            console.log('handleTrackUnsubscribed', track);
            // remove tracks from all attached elements
            track.detach();
            this.removeComponent(participant.identity, track);
        },

        handleLocalTrackUnpublished(trackPublication, participant) {
            const track = trackPublication.track;
            console.log('handleLocalTrackUnpublished sid=', track.sid, "kind=", track.kind);
            console.debug('handleLocalTrackUnpublished', trackPublication, "track", track);

            // when local tracks are ended, update UI to remove them from rendering
            track.detach();
            this.removeComponent(participant.identity, track);
        },
        removeComponent(userIdentity, track) {
            for (const component of this.userVideoComponents.getByUser(userIdentity)) {
                console.debug("For removal checking component=", component, "against", track);
                if (component.getVideoStreamId() == track.sid || component.getAudioStreamId() == track.sid) {
                    console.log("Removing component=", component.getId());
                    try {
                        this.videoContainerDiv.removeChild(component.$el);
                        component.$destroy();
                    } catch (e) {
                        console.debug("Something wrong on removing child", e, component.$el, this.videoContainerDiv);
                    }
                    this.userVideoComponents.removeComponentForUser(userIdentity, component);
                }
            }
        },

        handleActiveSpeakerChange(speakers) {
            console.debug("handleActiveSpeakerChange", speakers);

            for (const speaker of speakers) {
                const userIdentity = speaker.identity;
                const tracksSids = [...speaker.audioTracks.keys()];
                const userComponents = this.userVideoComponents.getByUser(userIdentity);
                for (const component of userComponents) {
                    const audioStreamId = component.getAudioStreamId();
                    console.debug("Track sids", tracksSids, " component audio stream id", audioStreamId);
                    if (tracksSids.includes(component.getAudioStreamId())) {
                        component.setSpeakingWithTimeout(1000);
                    }
                }
            }
        },

        handleDisconnect() {
            console.log('disconnected from room');

            // handles kick
            if (this.$route.name == videochat_name && !this.inRestarting) {
                console.log('Handling kick');

                const routerNewState = { name: chat_name, params: { leavingVideoAcceptableParam: true } };
                this.navigateToWithPreservingSearchStringInQuery(routerNewState);
            }
        },

        async setConfig() {
            await this.initServerData()
        },

        handleTrackMuted(trackPublication, participant) {
            const participantIdentityString = participant.identity;
            const components = this.userVideoComponents.getByUser(participantIdentityString);
            const matchedVideoComponents = components.filter(e => trackPublication.trackSid == e.getVideoStreamId());
            const matchedAudioComponents = components.filter(e => trackPublication.trackSid == e.getAudioStreamId());
            for (const component of matchedVideoComponents) {
                component.setVideoMute(trackPublication.isMuted);
            }
            for (const component of matchedAudioComponents) {
                component.setDisplayAudioMute(trackPublication.isMuted);
            }
        },

        async tryRestartVideoDevice() {
            this.inRestarting = true;
            for (const publication of this.room.localParticipant.tracks.values()) {
                this.room.localParticipant.unpublishTrack(publication.track, true);
            }
            await this.createLocalMediaTracks(null, null);
            bus.$emit(VIDEO_PARAMETERS_CHANGED);
            this.inRestarting = false;
        },

        async startRoom() {
            try {
                await this.setConfig();
            } catch (e) {
                this.setError(e, "Error during fetching config");
            }

            console.log("Creating room with dynacast", this.roomDynacast, "adaptiveStream", this.roomAdaptiveStream);

            // creates a new room with options
            this.room = new Room({
                // automatically manage subscribed video quality
                adaptiveStream: this.roomAdaptiveStream,

                // optimize publishing bandwidth and CPU for simulcasted tracks
                dynacast: this.roomDynacast,
            });

            // set up event listeners
            this.room
                .on(RoomEvent.TrackSubscribed, (track, publication, participant) => {
                    try {
                        console.log("TrackPublished to room.name", this.room.name);
                        console.debug("TrackPublished to room", this.room);
                        this.drawNewComponentOrInsertIntoExisting(participant, [publication], this.getOnScreenPosition(publication), null);
                    } catch (e) {
                        this.setError(e, "Error during reacting on remote track published");
                    }
                })
                .on(RoomEvent.TrackUnsubscribed, this.handleTrackUnsubscribed)
                .on(RoomEvent.ActiveSpeakersChanged, this.handleActiveSpeakerChange)
                .on(RoomEvent.LocalTrackUnpublished, this.handleLocalTrackUnpublished)
                .on(RoomEvent.LocalTrackPublished, () => {
                    try {
                        console.log("LocalTrackPublished to room.name", this.room.name);
                        console.debug("LocalTrackPublished to room", this.room);

                        const localVideoProperties = {
                            localParticipant: this.room.localParticipant
                        };
                        const participantTracks = this.room.localParticipant.getTracks();
                        this.drawNewComponentOrInsertIntoExisting(this.room.localParticipant, participantTracks, first, localVideoProperties);
                    } catch (e) {
                        this.setError(e, "Error during reacting on local track published");
                    }
                })
                .on(RoomEvent.TrackMuted, this.handleTrackMuted)
                .on(RoomEvent.TrackUnmuted, this.handleTrackMuted)
                .on(RoomEvent.Reconnecting, () => {
                    this.setWarning("Reconnecting to video server")
                })
                .on(RoomEvent.Reconnected, () => {
                    this.setOk("Successfully reconnected to video server")
                })
                .on(RoomEvent.Disconnected, this.handleDisconnect)
                .on(RoomEvent.SignalConnected, () => {
                    this.createLocalMediaTracks(null, null);
                })
            ;

            let token;
            try {
                token = await axios.get(`/api/video/${this.chatId}/token`).then(response => response.data.token);
                console.debug("Got video token", token);
            } catch (e) {
                this.setError(e, "Error during getting token");
                return;
            }

            const retryOptions = {
                delay: 200,
                maxAttempts: 5,
            };
            try {
                this.inRestarting = true;
                await retry(async (context) => {
                    const res = await this.room.connect(getWebsocketUrlPrefix() + '/api/livekit', token, {
                        // subscribe to other participants automatically
                        autoSubscribe: true,
                    });
                    console.log('connected to room', this.room.name);
                    return res
                }, retryOptions);
                this.inRestarting = false;
            } catch (e) {
                // If the max number of attempts was exceeded then `err`
                // will be the last error that was thrown.
                //
                // If error is due to timeout then `err.code` will be the
                // string `ATTEMPT_TIMEOUT`.
                this.setError(e, "Error during connecting to room");
            }
        },
        getOnScreenPosition(publication) {
            if (publication.trackName.startsWith("track_video__screen_true")) {
                return second
            }
            return last
        },

        async stopRoom() {
            console.log('Stopping room');
            await this.room.disconnect();
            this.room = null;
        },

        async createLocalMediaTracks(videoId, audioId, isScreen) {
            let tracks = [];

            try {
                const videoResolution = VideoPresets[this.videoResolution].resolution;
                const screenResolution = VideoPresets[this.screenResolution].resolution;
                const audioIsPresents = getStoredAudioDevicePresents();
                const videoIsPresents = getStoredVideoDevicePresents();

                if (!audioIsPresents && !videoIsPresents) {
                    console.warn("Not able to build local media stream, returning a successful promise");
                    bus.$emit(VIDEO_PARAMETERS_CHANGED, {error: 'No media configured'});
                    return Promise.reject('No media configured');
                }

                console.info(
                    "Creating media tracks", "audioId", audioId, "videoid", videoId,
                    "videoResolution", videoResolution, "screenResolution", screenResolution,
                    "videoSimulcast", this.videoSimulcast, "screenSimulcast", this.screenSimulcast,
                );

                if (isScreen) {
                    tracks = await createLocalScreenTracks({
                        audio: audioIsPresents,
                        resolution: screenResolution
                    });
                } else {
                    tracks = await createLocalTracks({
                        audio: audioIsPresents ? {
                            deviceId: audioId,
                            echoCancellation: true,
                            noiseSuppression: true,
                        } : false,
                        video: videoIsPresents ? {
                            deviceId: videoId,
                            resolution: videoResolution
                        } : false
                    })
                }
            } catch (e) {
                this.setError(e, "Error during creating local tracks");
            }

            try {
                const isMobileFirefox = isMobileFireFox();
                console.debug("isMobileFirefox = ", isMobileFirefox, " in case Mobile Firefox simulcast for video tracks will be disabled");
                for (const track of tracks) {
                    const normalizedScreen = !!isScreen;
                    const trackName = "track_" + track.kind + "__screen_" + normalizedScreen + "_" + this.getNewId();
                    const simulcast = !isMobileFirefox && (normalizedScreen ? this.screenSimulcast : this.videoSimulcast);
                    console.log(`Publishing local ${track.kind} screen=${normalizedScreen} track with name ${trackName} and simulcast ${simulcast}`);
                    const publication = await this.room.localParticipant.publishTrack(track, {
                        name: trackName,
                        simulcast: simulcast,
                    });
                    if (track.kind == 'audio' && defaultAudioMute) {
                        await publication.mute();
                    }
                    console.info("Published track sid=", track.sid, " kind=", track.kind);
                }
            } catch (e) {
                this.setError(e, "Error during publishing local tracks");
            }
        },
        onAddScreenSource() {
            this.createLocalMediaTracks(null, null, true);
        },
    },
    computed: {
        ...mapGetters({
            inRecordingProcess: GET_SHOW_RECORD_STOP_BUTTON,
            canMakeRecord: GET_CAN_MAKE_RECORD,
        })
    },
    async mounted() {
        this.chatId = this.$route.params.id;

        this.$store.commit(SET_SHOW_CALL_BUTTON, false);
        this.$store.commit(SET_SHOW_HANG_BUTTON, true);

        if (!this.inRecordingProcess && this.canMakeRecord) {
            this.$store.commit(SET_SHOW_RECORD_START_BUTTON, true);
            this.$store.commit(SET_SHOW_RECORD_STOP_BUTTON, false);
        }

        this.videoContainerDiv = document.getElementById("video-container");

        this.startRoom();
    },
    beforeDestroy() {
        axios.put(`/api/video/${this.chatId}/dial/stop`);
        this.stopRoom().then(()=>{
            console.log("Cleaning videoContainerDiv");
            this.videoContainerDiv = null;
            this.inRestarting = false;
        });

        this.$store.commit(SET_SHOW_CALL_BUTTON, true);
        this.$store.commit(SET_SHOW_HANG_BUTTON, false);
        this.$store.commit(SET_VIDEO_CHAT_USERS_COUNT, 0);
        this.$store.commit(SET_SHOW_RECORD_START_BUTTON, false);
    },
    created() {
        bus.$on(ADD_VIDEO_SOURCE, this.createLocalMediaTracks);
        bus.$on(ADD_SCREEN_SOURCE, this.onAddScreenSource);
        bus.$on(REQUEST_CHANGE_VIDEO_PARAMETERS, this.tryRestartVideoDevice);
    },
    destroyed() {
        bus.$off(ADD_VIDEO_SOURCE, this.createLocalMediaTracks);
        bus.$off(ADD_SCREEN_SOURCE, this.onAddScreenSource);
        bus.$off(REQUEST_CHANGE_VIDEO_PARAMETERS, this.tryRestartVideoDevice);
    }
}

</script>

<style lang="stylus" scoped>
#video-container {
    display: flex;
    flex-direction: row;
    overflow-x: scroll;
    overflow-y: hidden;
    scrollbar-width: none;
    //scroll-snap-align width
    //scroll-padding 0
    height 100%
    width 100%
    //object-fit: contain;
    //box-sizing: border-box
}

</style>