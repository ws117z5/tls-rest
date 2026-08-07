import axios from "axios";
import Logs from "@controllers/log"
import Config from "@controllers/config"

class Request {
    static apiListRequest = (url, that) => {
        return axios.get(Config.serverURL +  url)
                .then(res => {
                    //console.log(res.data)
                    //const users = res.data.children.map(obj => obj.data);

                    let state = {Data: [], Fieldset: [], loading: false};
                    if (typeof res.data.Data !== "undefined") {
                        state.Data = res.data.Data
                    }

                    if (typeof res.data.Fieldset !== "undefined") {
                        state.Fieldset = res.data.Fieldset
                    }

                    that.setState(state);
                });
    }

    static apiCall = (url, params) => {
        return axios.post(Config.serverURL +  url, params);
    }

    static apiRequest = (url, params = {}) => {
        return axios.get(Config.serverURL +  url, { params })
    }
}

export default Request