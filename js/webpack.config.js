const path = require('path');
const webpack = require('webpack');
const TsconfigPathsPlugin = require('tsconfig-paths-webpack-plugin');
const { BundleAnalyzerPlugin } = require('webpack-bundle-analyzer');

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
    '@css': path.resolve(__dirname, '../css') 
  },
  plugins: [
    new TsconfigPathsPlugin({
      configFile: path.resolve(__dirname, './tsconfig.json')
    })
  ]
};// Add HMR plugin for dev

module.exports = [
  {
    name: 'dev',
    mode: 'development',
    entry: './src/app.tsx',
    watch: true,
    resolve: commonResolve, // 👈 Shared here
    output: {
      path: path.resolve(__dirname, 'dist'),
      filename: '[name].js',
    },
    module: {
      rules: commonRules,
    },
    plugins: [
      new BundleAnalyzerPlugin({
        analyzerPort: 'auto', // Automatically uses next available port if 8888 is busy
      })
    ],
    optimization: {
      emitOnErrors: false,
    },
    devtool: 'source-map',
    output: {
      path: path.resolve(__dirname, 'dist'),
      filename: '[name].[contenthash].js',
      clean: true, // Empties /dist before every build
    },
  },
  {
    name: 'prod',
    mode: 'production',
    entry: './src/app.tsx',
    resolve: commonResolve, // 👈 Shared here
    module: {
      rules: commonRules,
    },
    optimization: {
    runtimeChunk: 'single', // Extracts Webpack runtime into a tiny separate file
    splitChunks: {
      chunks: 'all',
      maxInitialRequests: 5, // Prevents loading everything in a single massive vendor chunk
      cacheGroups: {
        defaultVendors: {
          test: /[\\/]node_modules[\\/]/,
          priority: -10,
          reuseExistingChunk: true,
        },
        default: {
          minChunks: 2,
          priority: -20,
          reuseExistingChunk: true,
        },
      },
    },
  },
    externals: {},
    devtool: 'source-map',
    output: {
      path: path.resolve(__dirname, 'dist'),
      filename: '[name].[contenthash].js',
      clean: true, // Empties /dist before every build
    },
  },
];