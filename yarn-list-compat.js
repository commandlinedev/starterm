#!/usr/bin/env node

// Temporary script to provide yarn list compatibility for electron-builder
// This script simulates the output that electron-builder expects from yarn list

const fs = require('fs');
const path = require('path');

// Read package.json to get dependencies
const packageJsonPath = path.join(process.cwd(), 'package.json');
const packageJson = JSON.parse(fs.readFileSync(packageJsonPath, 'utf8'));

// Create a minimal output structure that electron-builder expects
const output = {
  type: 'list',
  data: {
    type: 'list',
    trees: []
  }
};

// Add production dependencies
if (packageJson.dependencies) {
  Object.keys(packageJson.dependencies).forEach(dep => {
    output.data.trees.push({
      name: dep,
      children: [],
      hint: null,
      color: null,
      depth: 0
    });
  });
}

console.log(JSON.stringify(output));
