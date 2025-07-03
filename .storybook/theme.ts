import { create } from "@storybook/theming";

export const light = create({
    base: "light",
    brandTitle: "Star Terminal Storybook",
    brandUrl: "https://docs.starterm.dev/storybook/",
    brandImage: "./assets/star-light.png",
    brandTarget: "_self",
});

export const dark = create({
    base: "dark",
    brandTitle: "Star Terminal Storybook",
    brandUrl: "https://docs.starterm.dev/storybook/",
    brandImage: "./assets/star-dark.png",
    brandTarget: "_self",
});
