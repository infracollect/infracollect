# Website

## Overview

The website is built with Astro and Starlight.

### Running the website

```bash
npm run dev
```

### Building the website

```bash
npm run build
```

### Deploying the website

The website is deployed to GitHub Pages via a GitHub Actions workflow.

## Content

Follow Diataxis Framework for writing content:

Diataxis is a framework for creating documentation that **feels good to use** - documentation that has flow, anticipates
needs, and fits how humans actually interact with a craft.

**Important**: Diataxis is an approach, not a template. Don't create empty sections for
tutorials/how-to/reference/explanation just to have them. Create content that serves actual user needs, apply these
principles, and let structure emerge organically.

**Core insight**: Documentation serves practitioners in a domain of skill. What they need changes based on two
dimensions:

1. **Action vs Cognition** - doing things vs understanding things
2. **Acquisition vs Application** - learning vs working

These create exactly four documentation types:

- **Learning by doing** → Tutorials
- **Working to achieve a goal** → How-to Guides
- **Working and need facts** → Reference
- **Learning to understand** → Explanation

**Why exactly four**: These aren't arbitrary categories. The two dimensions create exactly four quarters - there cannot
be three or five. This is the complete territory of what documentation must cover.

## The Diataxis Compass (Your Primary Tool)

When uncertain which documentation type is needed, ask two questions:

**1. Does the content inform ACTION or COGNITION?**

- Action: practical steps, doing things
- Cognition: theoretical knowledge, understanding

**2. Does it serve ACQUISITION or APPLICATION of skill?**

- Acquisition: learning, study
- Application: working, getting things done

Then apply:

| Content Type | User Activity | Documentation Type |
| ------------ | ------------- | ------------------ |
| Action       | Acquisition   | **Tutorial**       |
| Action       | Application   | **How-to Guide**   |
| Cognition    | Application   | **Reference**      |
| Cognition    | Acquisition   | **Explanation**    |
