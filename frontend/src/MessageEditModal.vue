<template>
    <v-dialog v-model="show" fullscreen>
        <v-card>
            <v-toolbar
                dark
                color="indigo"
                dense
            >
                <v-btn
                    icon
                    dark
                    @click="closeModal()"
                >
                    <v-icon>mdi-close</v-icon>
                </v-btn>
                <v-toolbar-title>Editing message</v-toolbar-title>
            </v-toolbar>
            <div class="message-edit-dialog">
                <MessageEdit :chatId="chatId" full-height="true"/>
            </div>
        </v-card>
    </v-dialog>
</template>

<script>
    import bus, {CLOSE_EDIT_MESSAGE, OPEN_EDIT_MESSAGE, SET_EDIT_MESSAGE} from "@/bus";

    export default {
        data() {
            return {
                show: false,
            }
        },
        methods: {
            showModal(dto) {
                this.show = true;
                this.$nextTick(()=>{
                    bus.$emit(SET_EDIT_MESSAGE, dto);
                })
            },
            closeModal() {
                this.show = false;
            }
        },
        watch: {
            show(newValue) {
                if (!newValue) {
                    this.closeModal();
                }
            }
        },
        components: {
            MessageEdit: () => import("./MessageEdit"),
        },
        computed: {
            chatId() {
                return this.$route.params.id
            },
        },
        created() {
            bus.$on(OPEN_EDIT_MESSAGE, this.showModal);
            bus.$on(CLOSE_EDIT_MESSAGE, this.closeModal);
        },
        destroyed() {
            bus.$off(OPEN_EDIT_MESSAGE, this.showModal);
            bus.$off(CLOSE_EDIT_MESSAGE, this.closeModal);
        }
    }
</script>
