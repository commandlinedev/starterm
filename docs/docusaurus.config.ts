import type { Config } from "@docusaurus/types";
import rehypeHighlight from "rehype-highlight";
import { docOgRenderer } from "./src/renderer/image-renderers";

const baseUrl = process.env.EMBEDDED ? "/docsite/" : "/";

const config: Config = {
    title: "Star Terminal Documentation",
    tagline: "Level Up Your Terminal With Graphical Widgets",
    favicon: "img/logo/star-logo_appicon.svg",

    // Set the production url of your site here
    url: "https://commandlinedev.github.io/",
    // Set the /<baseUrl>/ pathname under which your site is served
    // For GitHub pages deployment, it is often '/<projectName>/'
    baseUrl,

    // GitHub pages deployment config.
    // If you aren't using GitHub pages, you don't need these.
    organizationName: "commandlinedev", // Usually your GitHub org/user name.
    projectName: "starterm-docs", // Usually your repo name.
    deploymentBranch: "main",

    onBrokenAnchors: "ignore",
    onBrokenLinks: "throw",
    onBrokenMarkdownLinks: "warn",
    trailingSlash: false,

    // Even if you don't use internationalization, you can use this field to set
    // useful metadata like html lang. For example, if your site is Chinese, you
    // may want to replace "en" with "zh-Hans".
    i18n: {
        defaultLocale: "en",
        locales: ["en"],
    },
    plugins: [
        [
            "content-docs",
            {
                path: "docs",
                routeBasePath: "/",
                exclude: ["features/**"],
                editUrl: !process.env.EMBEDDED ? "https://github.com/commandlinedev/starterm/edit/main/docs/" : undefined,
                rehypePlugins: [rehypeHighlight],
            } as import("@docusaurus/plugin-content-docs").Options,
        ],
        "ideal-image",
        [
            "@docusaurus/plugin-sitemap",
            {
                changefreq: "daily",
                filename: "sitemap.xml",
            },
        ],
        !process.env.EMBEDDED && [
            "@starterm/docusaurus-og",
            {
                path: "./preview-images", // relative to the build directory
                imageRenderers: {
                    "docusaurus-plugin-content-docs": docOgRenderer,
                },
            },
        ],
        "docusaurus-plugin-sass",
        "@docusaurus/plugin-svgr",
    ].filter((v) => v),
    themes: [
        ["classic", { customCss: "src/css/custom.scss" }],
        !process.env.EMBEDDED && "@docusaurus/theme-search-algolia",
    ].filter((v) => v),
    themeConfig: {
        docs: {
            sidebar: {
                hideable: false,
                autoCollapseCategories: false,
            },
        },
        colorMode: {
            defaultMode: "light",
            disableSwitch: false,
            respectPrefersColorScheme: true,
        },
        navbar: {
            logo: {
                src: "img/logo/star-light.png",
                srcDark: "img/logo/star-dark.png",
                href: "https://www.starterm.dev/",
            },
            hideOnScroll: true,
            items: [
                {
                    type: "doc",
                    position: "left",
                    docId: "index",
                    label: "Docs",
                },
                !process.env.EMBEDDED
                    ? [
                          {
                              position: "left",
                              href: "https://commandlinedev.github.io/storybook",
                              label: "Storybook",
                          },
                          {
                              href: "https://discord.gg/zUeP2aAjaP",
                              position: "right",
                              className: "header-link-custom custom-icon-discord",
                              "aria-label": "Discord invite",
                          },
                          {
                              href: "https://github.com/commandlinedev/starterm",
                              position: "right",
                              className: "header-link-custom custom-icon-github",
                              "aria-label": "GitHub repository",
                          },
                      ]
                    : [],
            ].flat(),
        },
        metadata: [
            {
                name: "keywords",
                content:
                    "terminal, developer, development, command, line, star, linux, macos, windows, connection, ssh, cli, starterm, documentation, docs, ai, graphical, widgets, remote, open, source, open-source, go, golang, react, typescript, javascript",
            },
            {
                name: "og:type",
                content: "website",
            },
            {
                name: "og:site_name",
                content: "Star Terminal Documentation",
            },
            {
                name: "application-name",
                content: "Star Terminal Documentation",
            },
            {
                name: "apple-mobile-web-app-title",
                content: "Star Terminal Documentation",
            },
        ],
        footer: {
            copyright: `Copyright © ${new Date().getFullYear()} Command Line Inc. Built with Docusaurus.`,
        },
        algolia: {
            appId: "B6A8512SN4",
            apiKey: "e879cd8663f109b2822cd004d9cd468c",
            indexName: "starterm",
        },
    },
    headTags: [
        {
            tagName: "link",
            attributes: {
                rel: "preload",
                as: "font",
                type: "font/woff2",
                "data-next-font": "size-adjust",
                href: `${baseUrl}fontawesome/webfonts/fa-sharp-regular-400.woff2`,
            },
        },
        {
            tagName: "link",
            attributes: {
                rel: "preload",
                as: "font",
                type: "font/woff2",
                "data-next-font": "size-adjust",
                href: `${baseUrl}fontawesome/webfonts/fa-sharp-solid-900.woff2`,
            },
        },
        {
            tagName: "link",
            attributes: {
                rel: "sitemap",
                type: "application/xml",
                title: "Sitemap",
                href: `${baseUrl}sitemap.xml`,
            },
        },
        !process.env.EMBEDDED && {
            tagName: "script",
            attributes: {
                defer: "true",
                "data-domain": "commandlinedev.github.io",
                src: "https://plausible.io/js/script.file-downloads.outbound-links.tagged-events.js",
            },
        },
    ].filter((v) => v),
    stylesheets: [
        `${baseUrl}fontawesome/css/fontawesome.min.css`,
        `${baseUrl}fontawesome/css/sharp-regular.min.css`,
        `${baseUrl}fontawesome/css/sharp-solid.min.css`,
    ],
    staticDirectories: ["static", "storybook"],
};

export default config;
