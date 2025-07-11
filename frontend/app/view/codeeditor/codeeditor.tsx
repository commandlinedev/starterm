// Copyright 2025, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

import { useOverrideConfigAtom } from "@/app/store/global";
import loader from "@monaco-editor/loader";
import { Editor, Monaco } from "@monaco-editor/react";
import type * as MonacoTypes from "monaco-editor/esm/vs/editor/editor.api";
import { configureMonacoYaml } from "monaco-yaml";
import React, { useMemo, useRef } from "react";

import { RpcApi } from "@/app/store/wshclientapi";
import { TabRpcClient } from "@/app/store/wshrpcutil";
import { boundNumber, makeConnRoute } from "@/util/util";
import editorWorker from "monaco-editor/esm/vs/editor/editor.worker?worker";
import cssWorker from "monaco-editor/esm/vs/language/css/css.worker?worker";
import htmlWorker from "monaco-editor/esm/vs/language/html/html.worker?worker";
import jsonWorker from "monaco-editor/esm/vs/language/json/json.worker?worker";
import tsWorker from "monaco-editor/esm/vs/language/typescript/ts.worker?worker";
import { SchemaEndpoints, getSchemaEndpointInfo } from "./schemaendpoints";
import ymlWorker from "./yamlworker?worker";

import "./codeeditor.scss";

// there is a global monaco variable (TODO get the correct TS type)
declare var monaco: Monaco;

window.MonacoEnvironment = {
    getWorker(_, label) {
        if (label === "json") {
            return new jsonWorker();
        }
        if (label === "css" || label === "scss" || label === "less") {
            return new cssWorker();
        }
        if (label === "yaml" || label === "yml") {
            return new ymlWorker();
        }
        if (label === "html" || label === "handlebars" || label === "razor") {
            return new htmlWorker();
        }
        if (label === "typescript" || label === "javascript") {
            return new tsWorker();
        }
        return new editorWorker();
    },
};

export async function loadMonaco() {
    loader.config({ paths: { vs: "monaco" } });
    await loader.init();

    monaco.editor.defineTheme("star-theme-dark", {
        base: "vs-dark",
        inherit: true,
        rules: [],
        colors: {
            "editor.background": "#00000000",
            "editorStickyScroll.background": "#00000055",
            "minimap.background": "#00000077",
            focusBorder: "#00000000",
        },
    });
    monaco.editor.defineTheme("star-theme-light", {
        base: "vs",
        inherit: true,
        rules: [],
        colors: {
            "editor.background": "#fefefe",
            focusBorder: "#00000000",
        },
    });
    configureMonacoYaml(monaco, {
        validate: true,
        schemas: [],
    });
    // Disable default validation errors for typescript and javascript
    monaco.languages.typescript.typescriptDefaults.setDiagnosticsOptions({
        noSemanticValidation: true,
    });
    const schemas = await Promise.all(SchemaEndpoints.map((endpoint) => getSchemaEndpointInfo(endpoint)));
    monaco.languages.json.jsonDefaults.setDiagnosticsOptions({
        validate: true,
        allowComments: false, // Set to true if you want to allow comments in JSON
        enableSchemaRequest: true,
        schemas,
    });
}

function defaultEditorOptions(): MonacoTypes.editor.IEditorOptions {
    const opts: MonacoTypes.editor.IEditorOptions = {
        scrollBeyondLastLine: false,
        fontSize: 12,
        fontFamily: "Hack",
        smoothScrolling: true,
        scrollbar: {
            useShadows: false,
            verticalScrollbarSize: 5,
            horizontalScrollbarSize: 5,
        },
        minimap: {
            enabled: true,
        },
        stickyScroll: {
            enabled: false,
        },
    };
    return opts;
}

interface CodeEditorProps {
    blockId: string;
    text: string;
    filename: string;
    fileinfo: FileInfo;
    language?: string;
    meta?: MetaType;
    onChange?: (text: string) => void;
    onMount?: (monacoPtr: MonacoTypes.editor.IStandaloneCodeEditor, monaco: Monaco) => () => void;
}

export function CodeEditor({ blockId, text, language, filename, fileinfo, meta, onChange, onMount }: CodeEditorProps) {
    const divRef = useRef<HTMLDivElement>(null);
    const unmountRef = useRef<() => void>(null);
    const minimapEnabled = useOverrideConfigAtom(blockId, "editor:minimapenabled") ?? false;
    const stickyScrollEnabled = useOverrideConfigAtom(blockId, "editor:stickyscrollenabled") ?? false;
    const wordWrap = useOverrideConfigAtom(blockId, "editor:wordwrap") ?? false;
    const fontSize = boundNumber(useOverrideConfigAtom(blockId, "editor:fontsize"), 6, 64);
    const theme = "star-theme-dark";
    const [absPath, setAbsPath] = React.useState("");

    React.useEffect(() => {
        return () => {
            // unmount function
            if (unmountRef.current) {
                unmountRef.current();
            }
        };
    }, []);

    React.useEffect(() => {
        const inner = async () => {
            try {
                const fileInfo = await RpcApi.RemoteFileJoinCommand(TabRpcClient, [filename], {
                    route: makeConnRoute(meta.connection ?? ""),
                });
                setAbsPath(fileInfo.path);
            } catch (e) {
                setAbsPath(filename);
            }
        };
        inner();
    }, [filename]);

    React.useEffect(() => {
        console.log("abspath is", absPath);
    }, [absPath]);

    function handleEditorChange(text: string, ev: MonacoTypes.editor.IModelContentChangedEvent) {
        if (onChange) {
            onChange(text);
        }
    }

    function handleEditorOnMount(editor: MonacoTypes.editor.IStandaloneCodeEditor, monaco: Monaco) {
        if (onMount) {
            unmountRef.current = onMount(editor, monaco);
        }
    }

    const editorOpts = useMemo(() => {
        const opts = defaultEditorOptions();
        opts.readOnly = fileinfo.readonly;
        opts.minimap.enabled = minimapEnabled;
        opts.stickyScroll.enabled = stickyScrollEnabled;
        opts.wordWrap = wordWrap ? "on" : "off";
        opts.fontSize = fontSize;
        return opts;
    }, [minimapEnabled, stickyScrollEnabled, wordWrap, fontSize, fileinfo.readonly]);

    return (
        <div className="code-editor-wrapper">
            <div className="code-editor" ref={divRef}>
                <Editor
                    theme={theme}
                    value={text}
                    options={editorOpts}
                    onChange={handleEditorChange}
                    onMount={handleEditorOnMount}
                    path={absPath}
                    language={language}
                />
            </div>
        </div>
    );
}
