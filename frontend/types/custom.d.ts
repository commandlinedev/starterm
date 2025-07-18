// Copyright 2025, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

import { type Placement } from "@floating-ui/react";
import type * as jotai from "jotai";
import type * as rxjs from "rxjs";

declare global {
    type GlobalAtomsType = {
        clientId: jotai.Atom<string>; // readonly
        client: jotai.Atom<Client>; // driven from WOS
        uiContext: jotai.Atom<UIContext>; // driven from windowId, tabId
        starWindow: jotai.Atom<StarWindow>; // driven from WOS
        workspace: jotai.Atom<Workspace>; // driven from WOS
        fullConfigAtom: jotai.PrimitiveAtom<FullConfigType>; // driven from WOS, settings -- updated via WebSocket
        settingsAtom: jotai.Atom<SettingsType>; // derrived from fullConfig
        tabAtom: jotai.Atom<Tab>; // driven from WOS
        staticTabId: jotai.Atom<string>;
        isFullScreen: jotai.PrimitiveAtom<boolean>;
        controlShiftDelayAtom: jotai.PrimitiveAtom<boolean>;
        prefersReducedMotionAtom: jotai.Atom<boolean>;
        updaterStatusAtom: jotai.PrimitiveAtom<UpdaterStatus>;
        typeAheadModalAtom: jotai.PrimitiveAtom<TypeAheadModalType>;
        modalOpen: jotai.PrimitiveAtom<boolean>;
        allConnStatus: jotai.Atom<ConnStatus[]>;
        flashErrors: jotai.PrimitiveAtom<FlashErrorType[]>;
        notifications: jotai.PrimitiveAtom<NotificationType[]>;
        notificationPopoverMode: jotia.atom<boolean>;
        reinitVersion: jotai.PrimitiveAtom<number>;
        isTermMultiInput: jotai.PrimitiveAtom<boolean>;
    };

    type WritableStarObjectAtom<T extends StarObj> = jotai.WritableAtom<T, [value: T], void>;

    type ThrottledValueAtom<T> = jotai.WritableAtom<T, [update: jotai.SetStateAction<T>], void>;

    type AtomWithThrottle<T> = {
        currentValueAtom: jotai.Atom<T>;
        throttledValueAtom: ThrottledValueAtom<T>;
    };

    type DebouncedValueAtom<T> = jotai.WritableAtom<T, [update: jotai.SetStateAction<T>], void>;

    type AtomWithDebounce<T> = {
        currentValueAtom: jotai.Atom<T>;
        debouncedValueAtom: DebouncedValueAtom<T>;
    };

    type SplitAtom<Item> = Atom<Atom<Item>[]>;
    type WritableSplitAtom<Item> = WritableAtom<PrimitiveAtom<Item>[], [SplitAtomAction<Item>], void>;

    type TabLayoutData = {
        blockId: string;
    };

    type StarInitOpts = {
        tabId: string;
        clientId: string;
        windowId: string;
        activate: boolean;
    };

    type ElectronApi = {
        getAuthKey(): string;
        getIsDev(): boolean;
        getCursorPoint: () => Electron.Point;
        getPlatform: () => NodeJS.Platform;
        getEnv: (varName: string) => string;
        getUserName: () => string;
        getHostName: () => string;
        getDataDir: () => string;
        getConfigDir: () => string;
        getWebviewPreload: () => string;
        getAboutModalDetails: () => AboutModalDetails;
        getDocsiteUrl: () => string;
        showContextMenu: (workspaceId: string, menu?: ElectronContextMenuItem[]) => void;
        onContextMenuClick: (callback: (id: string) => void) => void;
        onNavigate: (callback: (url: string) => void) => void;
        onIframeNavigate: (callback: (url: string) => void) => void;
        downloadFile: (path: string) => void;
        openExternal: (url: string) => void;
        onFullScreenChange: (callback: (isFullScreen: boolean) => void) => void;
        onUpdaterStatusChange: (callback: (status: UpdaterStatus) => void) => void;
        getUpdaterStatus: () => UpdaterStatus;
        getUpdaterChannel: () => string;
        installAppUpdate: () => void;
        onMenuItemAbout: (callback: () => void) => void;
        updateWindowControlsOverlay: (rect: Dimensions) => void;
        onReinjectKey: (callback: (starEvent: StarKeyboardEvent) => void) => void;
        setWebviewFocus: (focusedId: number) => void; // focusedId si the getWebContentsId of the webview
        registerGlobalWebviewKeys: (keys: string[]) => void;
        onControlShiftStateUpdate: (callback: (state: boolean) => void) => void;
        createWorkspace: () => void;
        switchWorkspace: (workspaceId: string) => void;
        deleteWorkspace: (workspaceId: string) => void;
        setActiveTab: (tabId: string) => void;
        createTab: () => void;
        closeTab: (workspaceId: string, tabId: string) => void;
        setWindowInitStatus: (status: "ready" | "star-ready") => void;
        onStarInit: (callback: (initOpts: StarInitOpts) => void) => void;
        sendLog: (log: string) => void;
        onQuicklook: (filePath: string) => void;
        openNativePath(filePath: string): void;
        captureScreenshot(rect: Electron.Rectangle): Promise<string>;
        setKeyboardChordMode: () => void;
    };

    type ElectronContextMenuItem = {
        id: string; // unique id, used for communication
        label: string;
        role?: string; // electron role (optional)
        type?: "separator" | "normal" | "submenu" | "checkbox" | "radio";
        submenu?: ElectronContextMenuItem[];
        checked?: boolean;
        visible?: boolean;
        enabled?: boolean;
        sublabel?: string;
    };

    type ContextMenuItem = {
        label?: string;
        type?: "separator" | "normal" | "submenu" | "checkbox" | "radio";
        role?: string; // electron role (optional)
        click?: () => void; // not required if role is set
        submenu?: ContextMenuItem[];
        checked?: boolean;
        visible?: boolean;
        enabled?: boolean;
        sublabel?: string;
    };

    type KeyPressDecl = {
        mods: {
            Cmd?: boolean;
            Option?: boolean;
            Shift?: boolean;
            Ctrl?: boolean;
            Alt?: boolean;
            Meta?: boolean;
        };
        key: string;
        keyType: string;
    };

    type SubjectWithRef<T> = rxjs.Subject<T> & { refCount: number; release: () => void };

    type HeaderElem =
        | IconButtonDecl
        | ToggleIconButtonDecl
        | HeaderText
        | HeaderInput
        | HeaderDiv
        | HeaderTextButton
        | ConnectionButton
        | MenuButton;

    type IconButtonCommon = {
        icon: string | React.ReactNode;
        iconColor?: string;
        iconSpin?: boolean;
        className?: string;
        title?: string;
        disabled?: boolean;
        noAction?: boolean;
    };

    type IconButtonDecl = IconButtonCommon & {
        elemtype: "iconbutton";
        click?: (e: React.MouseEvent<any>) => void;
        longClick?: (e: React.MouseEvent<any>) => void;
    };

    type ToggleIconButtonDecl = IconButtonCommon & {
        elemtype: "toggleiconbutton";
        active: jotai.WritableAtom<boolean, [boolean], void>;
    };

    type HeaderTextButton = {
        elemtype: "textbutton";
        text: string;
        className?: string;
        title?: string;
        onClick?: (e: React.MouseEvent<any>) => void;
    };

    type HeaderText = {
        elemtype: "text";
        text: string;
        ref?: React.MutableRefObject<HTMLDivElement>;
        className?: string;
        noGrow?: boolean;
        onClick?: (e: React.MouseEvent<any>) => void;
    };

    type HeaderInput = {
        elemtype: "input";
        value: string;
        className?: string;
        isDisabled?: boolean;
        ref?: React.MutableRefObject<HTMLInputElement>;
        onChange?: (e: React.ChangeEvent<HTMLInputElement>) => void;
        onKeyDown?: (e: React.KeyboardEvent<HTMLInputElement>) => void;
        onFocus?: (e: React.FocusEvent<HTMLInputElement>) => void;
        onBlur?: (e: React.FocusEvent<HTMLInputElement>) => void;
    };

    type HeaderDiv = {
        elemtype: "div";
        className?: string;
        children: HeaderElem[];
        onMouseOver?: (e: React.MouseEvent<any>) => void;
        onMouseOut?: (e: React.MouseEvent<any>) => void;
        onClick?: (e: React.MouseEvent<any>) => void;
    };

    type ConnectionButton = {
        elemtype: "connectionbutton";
        icon: string;
        text: string;
        iconColor: string;
        onClick?: (e: React.MouseEvent<any>) => void;
        connected: boolean;
    };

    type MenuItem = {
        label: string;
        icon?: string | React.ReactNode;
        subItems?: MenuItem[];
        onClick?: (e: React.MouseEvent<any>) => void;
    };

    type MenuButtonProps = {
        items: MenuItem[];
        className?: string;
        text: string;
        title?: string;
        menuPlacement?: Placement;
    };

    type MenuButton = {
        elemtype: "menubutton";
    } & MenuButtonProps;

    type SearchAtoms = {
        searchValue: PrimitiveAtom<string>;
        resultsIndex: PrimitiveAtom<number>;
        resultsCount: PrimitiveAtom<number>;
        isOpen: PrimitiveAtom<boolean>;
        regex?: PrimitiveAtom<boolean>;
        caseSensitive?: PrimitiveAtom<boolean>;
        wholeWord?: PrimitiveAtom<boolean>;
    };

    declare type ViewComponentProps<T extends ViewModel> = {
        blockId: string;
        blockRef: React.RefObject<HTMLDivElement>;
        contentRef: React.RefObject<HTMLDivElement>;
        model: T;
    };

    declare type ViewComponent = React.FC<ViewComponentProps>;

    type ViewModelClass = new (blockId: string, nodeModel: BlockNodeModel) => ViewModel;

    interface ViewModel {
        // The type of view, used for identifying and rendering the appropriate component.
        viewType: string;

        // Icon representing the view, can be a string or an IconButton declaration.
        viewIcon?: jotai.Atom<string | IconButtonDecl>;

        // Display name for the view, used in UI headers.
        viewName?: jotai.Atom<string>;

        // Optional header text or elements for the view.
        viewText?: jotai.Atom<string | HeaderElem[]>;

        // Icon button displayed before the title in the header.
        preIconButton?: jotai.Atom<IconButtonDecl>;

        // Icon buttons displayed at the end of the block header.
        endIconButtons?: jotai.Atom<IconButtonDecl[]>;

        // Background styling metadata for the block.
        blockBg?: jotai.Atom<MetaType>;

        noHeader?: jotai.Atom<boolean>;

        // Whether the block manages its own connection (e.g., for remote access).
        manageConnection?: jotai.Atom<boolean>;

        // If true, filters out 'nowsh' connections (when managing connections)
        filterOutNowsh?: jotai.Atom<boolean>;

        // if true, show s3 connections in picker
        showS3?: jotai.Atom<boolean>;

        // If true, removes padding inside the block content area.
        noPadding?: jotai.Atom<boolean>;

        // Atoms used for managing search functionality within the block.
        searchAtoms?: SearchAtoms;

        // The main view component associated with this ViewModel.
        viewComponent: ViewComponent<ViewModel>;

        // Function to determine if this is a basic terminal block.
        isBasicTerm?: (getFn: jotai.Getter) => boolean;

        // Returns menu items for the settings dropdown.
        getSettingsMenuItems?: () => ContextMenuItem[];

        // Attempts to give focus to the block, returning true if successful.
        giveFocus?: () => boolean;

        // Handles keydown events within the block.
        keyDownHandler?: (e: StarKeyboardEvent) => boolean;

        // Cleans up resources when the block is disposed.
        dispose?: () => void;
    }

    type UpdaterStatus = "up-to-date" | "checking" | "downloading" | "ready" | "error" | "installing";

    // jotai doesn't export this type :/
    type Loadable<T> = { state: "loading" } | { state: "hasData"; data: T } | { state: "hasError"; error: unknown };

    interface Dimensions {
        width: number;
        height: number;
        left: number;
        top: number;
    }

    type TypeAheadModalType = { [key: string]: boolean };

    interface AboutModalDetails {
        version: string;
        buildTime: number;
    }

    type BlockComponentModel = {
        openSwitchConnection?: () => void;
        viewModel: ViewModel;
    };

    type ConnStatusType = "connected" | "connecting" | "disconnected" | "error" | "init";

    interface SuggestionBaseItem {
        label: string;
        value: string;
        icon?: string | React.ReactNode;
    }

    interface SuggestionConnectionItem extends SuggestionBaseItem {
        status: ConnStatusType;
        iconColor: string;
        onSelect?: (_: string) => void;
        current?: boolean;
    }

    interface SuggestionConnectionScope {
        headerText?: string;
        items: SuggestionConnectionItem[];
    }

    type SuggestionsType = SuggestionConnectionItem | SuggestionConnectionScope;

    type MarkdownResolveOpts = {
        connName: string;
        baseDir: string;
    };

    type FlashErrorType = {
        id: string;
        icon: string;
        title: string;
        message: string;
        expiration: number;
    };

    export type NotificationActionType = {
        label: string;
        actionKey: string;
        rightIcon?: string;
        color?: "green" | "grey";
        disabled?: boolean;
    };

    export type NotificationType = {
        id?: string;
        icon: string;
        title: string;
        message: string;
        timestamp: string;
        expiration?: number;
        hidden?: boolean;
        actions?: NotificationActionType[];
        persistent?: boolean;
        type?: "error" | "update" | "info" | "warning";
    };

    interface AbstractWshClient {
        recvRpcMessage(msg: RpcMessage): void;
    }

    type ClientRpcEntry = {
        reqId: string;
        startTs: number;
        command: string;
        msgFn: (msg: RpcMessage) => void;
    };

    type TimeSeriesMeta = {
        name?: string;
        color?: string;
        label?: string;
        maxy?: string | number;
        miny?: string | number;
        decimalPlaces?: number;
    };

    interface SuggestionRequestContext {
        widgetid: string;
        reqnum: number;
        dispose?: boolean;
    }

    type SuggestionsFnType = (query: string, reqContext: SuggestionRequestContext) => Promise<FetchSuggestionsResponse>;

    type DraggedFile = {
        uri: string;
        absParent: string;
        relName: string;
        isDir: boolean;
    };

    type ErrorButtonDef = {
        text: string;
        onClick: () => void;
    };

    type ErrorMsg = {
        status: string;
        text: string;
        level?: "error" | "warning";
        buttons?: Array<ErrorButtonDef>;
        closeAction?: () => void;
        showDismiss?: boolean;
    };
}

export {};
