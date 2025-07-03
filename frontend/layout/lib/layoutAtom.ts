// Copyright 2025, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

import { WOS } from "@/app/store/global";
import { Atom, atom, Getter } from "jotai";
import { LayoutTreeState, WritableLayoutTreeStateAtom } from "./types";

const layoutStateAtomMap: WeakMap<Atom<Tab>, WritableLayoutTreeStateAtom> = new WeakMap();

function getLayoutStateAtomFromTab(tabAtom: Atom<Tab>, get: Getter): WritableStarObjectAtom<LayoutState> {
    const tabData = get(tabAtom);
    if (!tabData) return;
    const layoutStateOref = WOS.makeORef("layout", tabData.layoutstate);
    const layoutStateAtom = WOS.getStarObjectAtom<LayoutState>(layoutStateOref);
    return layoutStateAtom;
}

export function withLayoutTreeStateAtomFromTab(tabAtom: Atom<Tab>): WritableLayoutTreeStateAtom {
    if (layoutStateAtomMap.has(tabAtom)) {
        return layoutStateAtomMap.get(tabAtom);
    }
    const generationAtom = atom(1);
    const treeStateAtom: WritableLayoutTreeStateAtom = atom(
        (get) => {
            const stateAtom = getLayoutStateAtomFromTab(tabAtom, get);
            if (!stateAtom) return;
            const layoutStateData = get(stateAtom);
            const layoutTreeState: LayoutTreeState = {
                rootNode: layoutStateData?.rootnode,
                focusedNodeId: layoutStateData?.focusednodeid,
                magnifiedNodeId: layoutStateData?.magnifiednodeid,
                pendingBackendActions: layoutStateData?.pendingbackendactions,
                generation: get(generationAtom),
            };
            return layoutTreeState;
        },
        (get, set, value) => {
            if (get(generationAtom) < value.generation) {
                const stateAtom = getLayoutStateAtomFromTab(tabAtom, get);
                if (!stateAtom) return;
                const starObjVal = get(stateAtom);
                if (starObjVal == null) {
                    console.log("in withLayoutTreeStateAtomFromTab, starObjVal is null", value);
                    return;
                }
                starObjVal.rootnode = value.rootNode;
                starObjVal.magnifiednodeid = value.magnifiedNodeId;
                starObjVal.focusednodeid = value.focusedNodeId;
                starObjVal.leaforder = value.leafOrder; // only set leaforder, never get it, since this value is driven by the frontend
                starObjVal.pendingbackendactions = value?.pendingBackendActions?.length
                    ? value.pendingBackendActions
                    : undefined;
                set(generationAtom, value.generation);
                set(stateAtom, starObjVal);
            }
        }
    );
    layoutStateAtomMap.set(tabAtom, treeStateAtom);
    return treeStateAtom;
}
