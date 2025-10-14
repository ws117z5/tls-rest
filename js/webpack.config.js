const path = require('path');
const webpack = require('webpack');

module.exports = [{
    //test: /.tsx?$/,
    name: 'dev',
    entry: "./src/app.tsx", //path.resolve(__dirname, "src"),
    mode: 'development',
    watch: true,
    resolve: {
        // changed from extensions: [".js", ".jsx"]
        extensions: [".ts", ".tsx", ".js", ".jsx"]
    },
    module: {
      rules: [
        // changed from { test: /\.jsx?$/, use: { loader: 'babel-loader' }, exclude: /node_modules/ },
        { test: /\.(t|j)sx?$/, use: { loader: 'ts-loader' }, exclude: /node_modules/ },
  
        // addition - add source-map supportw
        { enforce: "pre", test: /\.js$/, exclude: /node_modules/, loader: "source-map-loader" },
        {
          test: /\.svg$/,
          use: ['@svgr/webpack', 'url-loader'],
        },
        {
          test: /\.(css|scss)$/i,
          use: ["style-loader", "css-loader", "sass-loader"],
        }

      ]
    },
    optimization: {
      emitOnErrors: false
    },

    devtool: "source-map",
}, 


{
    //test: /.jsx?$/,
    name: 'prod',
    entry: "./src/app.tsx", //path.resolve(__dirname, "src"),
    mode: 'production',
    resolve: {
      // changed from extensions: [".js", ".jsx"]
      extensions: [".ts", ".tsx", ".js", ".jsx"]
    },
    module: {
        rules: [
              { test: /\.(t|j)sx?$/, use: { loader: 'ts-loader' }, exclude: /node_modules/ },
    
              // addition - add source-map supportw
              { enforce: "pre", test: /\.js$/, exclude: /node_modules/, loader: "source-map-loader" },
              //test: /\.(t|j)sx?$/, use: { loader: 'ts-loader' }, exclude: /node_modules/ ,
              //loader: "ts-loader",
              //loader: "babel-loader",
              // use: {
              //     loader: "babel-loader"
              // }
              {
                test: /\.(css|scss)$/i,
                use: ["style-loader", "css-loader", "sass-loader"],
              }
        ]
    },
    externals: {
      //"react": "React",
      //"react-dom": "ReactDOM",
    },

    devtool: "source-map",
}];