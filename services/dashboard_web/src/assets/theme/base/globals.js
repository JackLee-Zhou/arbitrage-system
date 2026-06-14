import colors from "assets/theme/base/colors";

const { info, dark } = colors;

export default {
  html: {
    scrollBehavior: "smooth",
    background: dark.body,
  },
  body: {
    background: dark.body,
    backgroundImage: `
      radial-gradient(ellipse at 50% 0%, rgba(0,229,255,0.04) 0%, transparent 60%),
      radial-gradient(ellipse at 80% 100%, rgba(0,230,118,0.03) 0%, transparent 50%),
      linear-gradient(180deg, #070B14 0%, #0D1321 50%, #070B14 100%)
    `,
  },
  "*, *::before, *::after": {
    margin: 0,
    padding: 0,
  },
  "a, a:link, a:visited": {
    textDecoration: "none !important",
  },
  "a.link, .link, a.link:link, .link:link, a.link:visited, .link:visited": {
    color: `${dark.main} !important`,
    transition: "color 150ms ease-in !important",
  },
  "a.link:hover, .link:hover, a.link:focus, .link:focus": {
    color: `${info.main} !important`,
  },
};
