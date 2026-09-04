const path = require('path');
const webpack = require('webpack');
const TsconfigPathsPlugin = require('tsconfig-paths-webpack-plugin');

const commonRules = [
  {
    test: /\.(t|j)sx?$/,
    exclude: /node_modules/,
    use: {
      loader: 'swc-loader',
      options: {
        jsc: {
          parser: {
            syntax: 'typescript',
            tsx: true,
          },
          transform: {
            react: {
              runtime: 'automatic',
            },
          },
        },
      },
    },
  },
  {
    enforce: 'pre',
    test: /\.js$/,
    exclude: /node_modules/,
    loader: 'source-map-loader',
  },
  {
    test: /\.svg$/,
    use: ['@svgr/webpack', 'url-loader'],
  },
  {
    test: /\.(css|scss)$/i,
    use: ['style-loader', 'css-loader', 'sass-loader'],
  },
];

// Add plugins array to shared resolve config
const commonResolve = {
  extensions: ['.ts', '.tsx', '.js', '.jsx', '.css'],
  alias: {
    // Explicitly bind @css to the actual CSS directory
    '@css': path.resolve(__dirname, 'src/css') 
  },
  plugins: [
    new TsconfigPathsPlugin({
      configFile: path.resolve(__dirname, './tsconfig.json')
    })
  ]
};

module.exports = [
  {
    name: 'dev',
    mode: 'development',
    entry: './src/app.tsx',
    watch: true,
    resolve: commonResolve, // 👈 Shared here
    module: {
      rules: commonRules,
    },
    optimization: {
      emitOnErrors: false,
    },
    devtool: 'source-map',
  },
  {
    name: 'prod',
    mode: 'production',
    entry: './src/app.tsx',
    resolve: commonResolve, // 👈 Shared here
    module: {
      rules: commonRules,
    },
    externals: {},
    devtool: 'source-map',
  },
];