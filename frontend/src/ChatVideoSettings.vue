<template>
    <v-row justify="center">
        <v-dialog v-model="show" max-width="400" persistent>
            <v-card>
                <v-card-title>Settings</v-card-title>

                <v-container fluid>
                    <v-checkbox
                        v-model="video"
                        :label="`Video: ${video.toString()}`"
                    ></v-checkbox>
                    <v-checkbox
                        v-model="audio"
                        :label="`Audio: ${audio.toString()}`"
                    ></v-checkbox>
                </v-container>

                <v-card-actions class="pa-4">
                    <v-btn color="error" class="mr-4" @click="show=false">Close</v-btn>
                    <v-spacer/>
                </v-card-actions>
            </v-card>
        </v-dialog>
    </v-row>
</template>

<script>
import bus, {OPEN_VIDEO_SETTINGS_DIALOG, VIDEO_SETTINGS_AUDIO_CHANGED, VIDEO_SETTINGS_VIDEO_CHANGED} from "./bus";

    export default {
        data () {
            return {
                show: false,
                audio: true,
                video: true
            }
        },
        methods: {
            showModal() {
                this.$data.show = true;
            },
        },
        watch: {
            video(newVal) {
                console.log("New video", newVal);
                bus.$emit(VIDEO_SETTINGS_VIDEO_CHANGED, newVal);

            },
            audio(newVal) {
                console.log("New audio", newVal);
                bus.$emit(VIDEO_SETTINGS_AUDIO_CHANGED, newVal);
            }
        },
        created() {
            bus.$on(OPEN_VIDEO_SETTINGS_DIALOG, this.showModal);
        },
        destroyed() {
            bus.$off(OPEN_VIDEO_SETTINGS_DIALOG, this.showModal);
        },
    }
</script>