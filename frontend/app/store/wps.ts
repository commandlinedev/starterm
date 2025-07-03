// Copyright 2025, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

import { isBlank } from "@/util/util";
import { Subject } from "rxjs";
import { sendRawRpcMessage } from "./ws";

type StarEventSubject = {
    handler: (event: StarEvent) => void;
    scope?: string;
};

type StarEventSubjectContainer = StarEventSubject & {
    id: string;
};

type StarEventSubscription = StarEventSubject & {
    eventType: string;
};

type StarEventUnsubscribe = {
    id: string;
    eventType: string;
};

// key is "eventType" or "eventType|oref"
const fileSubjects = new Map<string, SubjectWithRef<WSFileEventData>>();
const starEventSubjects = new Map<string, StarEventSubjectContainer[]>();

function wpsReconnectHandler() {
    for (const eventType of starEventSubjects.keys()) {
        updateStarEventSub(eventType);
    }
}

function makeStarReSubCommand(eventType: string): RpcMessage {
    let subjects = starEventSubjects.get(eventType);
    if (subjects == null) {
        return { command: "eventunsub", data: eventType };
    }
    let subreq: SubscriptionRequest = { event: eventType, scopes: [], allscopes: false };
    for (const scont of subjects) {
        if (isBlank(scont.scope)) {
            subreq.allscopes = true;
            subreq.scopes = [];
            break;
        }
        subreq.scopes.push(scont.scope);
    }
    return { command: "eventsub", data: subreq };
}

function updateStarEventSub(eventType: string) {
    const command = makeStarReSubCommand(eventType);
    // console.log("updateStarEventSub", eventType, command);
    sendRawRpcMessage(command);
}

function starEventSubscribe(...subscriptions: StarEventSubscription[]): () => void {
    const unsubs: StarEventUnsubscribe[] = [];
    const eventTypeSet = new Set<string>();
    for (const subscription of subscriptions) {
        // console.log("starEventSubscribe", subscription);
        if (subscription.handler == null) {
            return;
        }
        const id: string = crypto.randomUUID();
        let subjects = starEventSubjects.get(subscription.eventType);
        if (subjects == null) {
            subjects = [];
            starEventSubjects.set(subscription.eventType, subjects);
        }
        const subcont: StarEventSubjectContainer = { id, handler: subscription.handler, scope: subscription.scope };
        subjects.push(subcont);
        unsubs.push({ id, eventType: subscription.eventType });
        eventTypeSet.add(subscription.eventType);
    }
    for (const eventType of eventTypeSet) {
        updateStarEventSub(eventType);
    }
    return () => starEventUnsubscribe(...unsubs);
}

function starEventUnsubscribe(...unsubscribes: StarEventUnsubscribe[]) {
    const eventTypeSet = new Set<string>();
    for (const unsubscribe of unsubscribes) {
        let subjects = starEventSubjects.get(unsubscribe.eventType);
        if (subjects == null) {
            return;
        }
        const idx = subjects.findIndex((s) => s.id === unsubscribe.id);
        if (idx === -1) {
            return;
        }
        subjects.splice(idx, 1);
        if (subjects.length === 0) {
            starEventSubjects.delete(unsubscribe.eventType);
        }
        eventTypeSet.add(unsubscribe.eventType);
    }

    for (const eventType of eventTypeSet) {
        updateStarEventSub(eventType);
    }
}

function getFileSubject(zoneId: string, fileName: string): SubjectWithRef<WSFileEventData> {
    const subjectKey = zoneId + "|" + fileName;
    let subject = fileSubjects.get(subjectKey);
    if (subject == null) {
        subject = new Subject<any>() as any;
        subject.refCount = 0;
        subject.release = () => {
            subject.refCount--;
            if (subject.refCount === 0) {
                subject.complete();
                fileSubjects.delete(subjectKey);
            }
        };
        fileSubjects.set(subjectKey, subject);
    }
    subject.refCount++;
    return subject;
}

function handleStarEvent(event: StarEvent) {
    // console.log("handleStarEvent", event);
    const subjects = starEventSubjects.get(event.event);
    if (subjects == null) {
        return;
    }
    for (const scont of subjects) {
        if (isBlank(scont.scope)) {
            scont.handler(event);
            continue;
        }
        if (event.scopes == null) {
            continue;
        }
        if (event.scopes.includes(scont.scope)) {
            scont.handler(event);
        }
    }
}

export { getFileSubject, handleStarEvent, starEventSubscribe, starEventUnsubscribe, wpsReconnectHandler };
