<template>
    <v-list-item
        dense
        class="pr-1 mr-1 pl-4"
        :class="{ highlight: highlight }"
    >
        <router-link :to="{ name: 'profileUser', params: { id: source.owner.id }}">
            <v-list-item-avatar v-if="source.owner && source.owner.avatar">
                <v-img :src="source.owner.avatar"></v-img>
            </v-list-item-avatar>
        </router-link>

        <v-list-item-content @click="onMessageClick(source)" @mousemove="onMessageMouseMove(source)">
            <v-container class="ma-0 pa-0 d-flex list-item-head">
                <router-link :to="{ name: 'profileUser', params: { id: source.owner.id }}">{{getOwner(source)}}</router-link><span class="with-space"> at </span>{{getDate(source)}}
                <v-icon class="mx-1 ml-2" v-if="source.fileItemUuid" @click="onFilesClicked(source.fileItemUuid)" small>mdi-file-download</v-icon>
                <v-icon class="mx-1" v-if="source.canEdit" color="error" @click="deleteMessage(source)" dark small>mdi-delete</v-icon>
                <v-icon class="mx-1" v-if="source.canEdit" color="primary" @click="editMessage(source)" dark small>mdi-lead-pencil</v-icon>
            </v-container>
            <v-list-item-content class="pre-formatted pa-0 ma-0 mt-1 message-item-text" v-html="source.text"></v-list-item-content>
        </v-list-item-content>
    </v-list-item>
</template>

<script>
    import axios from "axios";
    import bus, {CLOSE_SIMPLE_MODAL, OPEN_SIMPLE_MODAL, SET_EDIT_MESSAGE, OPEN_VIEW_FILES_DIALOG} from "./bus";
    import debounce from "lodash/debounce";
    import { format, parseISO, differenceInDays } from 'date-fns'

    export default {
        props: {
            index: { // index of current item
                type: Number
            },
            source: { // here is: {uid: 'unique_1', text: 'abc'}
                type: Object,
                default () {
                    return {}
                }
            }
        },
        computed: {
            highlight() {
                // item.owner.id === currentUser.id
                return false
            }
        },
        methods: {
            onMessageClick(dto) {
                this.centrifuge.send({payload: { chatId: this.chatId, messageId: dto.id}, "type": "message_read"})
            },
            onMessageMouseMove(item) {
                this.onMessageClick(item);
            },
            deleteMessage(dto){
                bus.$emit(OPEN_SIMPLE_MODAL, {
                    buttonName: 'Delete',
                    title: `Delete message #${dto.id}`,
                    text: `Are you sure to delete this message ?`,
                    actionFunction: ()=> {
                        axios.delete(`/api/chat/${this.chatId}/message/${dto.id}`)
                            .then(() => {
                                bus.$emit(CLOSE_SIMPLE_MODAL);
                            })
                    }
                });
            },
            editMessage(dto){
                const editMessageDto = {id: dto.id, text: dto.text, fileItemUuid: dto.fileItemUuid};
                bus.$emit(SET_EDIT_MESSAGE, editMessageDto);
            },
            getOwner(item) {
                return item.owner.login
            },
            getDate(item) {
                const parsedDate = parseISO(item.createDateTime);
                let formatString = 'HH:mm:ss';
                if (differenceInDays(new Date(), parsedDate) >= 1) {
                    formatString = 'd MMM yyyy, ' + formatString;
                }
                return `${format(parsedDate, formatString)}`
            },
            onFilesClicked(itemId) {
                bus.$emit(OPEN_VIEW_FILES_DIALOG, {chatId: this.chatId, fileItemUuid :itemId});
            }
        },
        created() {
            this.onMessageMouseMove = debounce(this.onMessageMouseMove, 1000, {leading:true, trailing:false});
        },
    }
</script>

<style lang="stylus">
  .list-item-head {
    color:rgba(0, 0, 0, .6);
    font-size: .8125rem;
    font-weight: 500;
    line-height: 1rem;
  }
  .message-item-text {
      display inline-block
      word-wrap break-word
      overflow-wrap break-word
  }
  .with-space {
      white-space: pre;
  }
  .highlight {
      background #cde3ff
  }
</style>