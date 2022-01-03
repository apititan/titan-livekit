<template>
    <div class="quillWrapper">
        <div ref="quillContainer" :id="id"></div>
        <input v-if="useCustomImageHandler" @change="emitImageInfo($event)" ref="fileInput" id="file-upload" type="file" style="display:none;">
    </div>
</template>

<script>
// https://github.com/davidroyer/vue2-editor/blob/master/src/components/VueEditor.vue
import VQuill from 'quill'
import merge from 'lodash/merge'
const Quill = VQuill;

export default {
    name: 'vue-editor',
    props: {
        value: String,
        id: {
            type: String,
            default: 'quill-container'
        },
        placeholder: String,
        disabled: Boolean,
        customModules: Array,
        editorToolbar: Array,
        editorOptions: {
            type: Object,
            default: function () {
                return {};
            }
        },
        useCustomImageHandler: {
            type: Boolean,
            default: false
        },
    },

    computed: {
        filteredInitialContent() {
            let content = this.value || ''
            return content.replace(/(<div)/igm, '<p').replace(/<\/div>/igm, '</p>');
        },

        imageResizeActive() {
            return this.quill.options.modules.imageResize !== undefined ? true : false
        }
    },

    data() {
        return {
            quill: null,
            editor: null,
            editorConfig: {},
            modules: {
                toolbar: this.editorToolbar,
            }
        }
    },

    mounted() {
        this.initializeVue2Editor()
        this.handleUpdatedEditor()
    },

    watch: {
        value (val) {
            if (val !=  this.editor.innerHTML && !this.quill.hasFocus()) {
                this.editor.innerHTML = val
            }
        },
        disabled(status) {
            this.quill.enable(!status);
        }
    },

    methods: {
        initializeVue2Editor() {
            this.prepareModules()
            this.setQuillElement()
            this.setEditorElement()
            this.handleDynamicStyles()
            this.checkForInitialContent()
            this.checkForCustomImageHandler()
            this.applyGoogleKeyboardWorkaround(this.quill)
        },

        setQuillElement() {
            let editorConfig = {
                debug: false,
                modules: this.modules,
                placeholder: this.placeholder ? this.placeholder : '',
                theme: 'snow',
                readOnly: this.disabled ? this.disabled : false,
            };
            this.prepareEditorConfig(editorConfig)
            this.quill = new Quill(this.$refs.quillContainer, editorConfig)
        },

        setEditorElement() {
            this.editor = document.querySelector(`#${this.id} .ql-editor`)
        },

        handleDynamicStyles() {
            if ( this.imageResizeActive ) {
                this.editor.classList.add('imageResizeActive');
            }
        },

        prepareModules() {
            this.registerCustomModules();
        },

        registerCustomModules() {
            if ( this.customModules !== undefined ) {
                this.customModules.forEach(customModule => {
                    Quill.register('modules/' + customModule.alias, customModule.module)
                })
            }
        },

        prepareEditorConfig(editorConfig) {
            if (Object.keys(this.editorOptions).length > 0 && this.editorOptions.constructor === Object) {
                if (this.editorOptions.modules && typeof this.editorOptions.modules.toolbar !== 'undefined') {
                    // We don't want to merge default toolbar with provided toolbar.
                    delete editorConfig.modules.toolbar;
                }
                merge(editorConfig, this.editorOptions);
            }
        },

        checkForInitialContent() {
            this.editor.innerHTML = this.filteredInitialContent
        },

        checkForCustomImageHandler() {
            this.useCustomImageHandler === true ? this.setupCustomImageHandler() : ''
        },

        setupCustomImageHandler() {
            let toolbar = this.quill.getModule('toolbar');
            toolbar.addHandler('image', this.customImageHandler);
        },

        handleUpdatedEditor() {
            this.quill.on('text-change', () => {
                this.$emit('input', this.editor.innerHTML)
            })
        },

        customImageHandler(image, callback) {
            this.$refs.fileInput.click();
        },

        emitImageInfo($event) {
            const resetUploader = function() {
                var uploader = document.getElementById('file-upload');
                uploader.value = '';
            }

            let file = $event.target.files[0]
            let Editor = this.quill
            let range = Editor.getSelection();
            let cursorLocation = range.index
            this.$emit('imageAdded', file, Editor, cursorLocation, resetUploader)
        },
        applyGoogleKeyboardWorkaround(editor) {
            try {
                if (!editor.applyGoogleKeyboardWorkaround) {
                    // https://github.com/quilljs/quill/issues/3240
                    console.log("applying gboard workaround");
                    editor.applyGoogleKeyboardWorkaround = true;
                    editor.on('editor-change', (eventName, ...args) => {
                        if (eventName === 'text-change') {
                            // args[0] will be delta
                            const ops = args[0].ops;
                            const oldSelection = editor.getSelection();
                            const oldPos = oldSelection?.index;
                            const oldSelectionLength = oldSelection ? oldSelection.length : 0;

                            if (ops[0].retain === undefined ||
                                !ops[1] ||
                                !ops[1].insert ||
                                !ops[1].insert ||
                                ops[1].insert !== '\n' ||
                                oldSelectionLength > 0) {
                                return;
                            }

                            setTimeout(() => {
                                const newPos = editor.getSelection().index;
                                if (newPos === oldPos) {
                                    console.log('Change selection bad pos');
                                    editor.setSelection(editor.getSelection().index + 1, 0);
                                }
                            }, 30);
                        }
                    });
                    console.log('gboard workaround has been successfully applied');
                }
            } catch(e) {
                console.log('error during applying gboard workaround');
                console.debug('error during applying gboard workaround', e);
            }
        }
    }
}
</script>
