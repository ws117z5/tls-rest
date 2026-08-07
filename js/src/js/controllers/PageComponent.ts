import React, { Component } from "react";
import Functional from "@controllers/functional";
import axios from "axios";
import Config from "@controllers/config";
import { WithRouterProps } from "@controllers/AbstractComponent";


export interface DefaultListProps {
  data: any; // Replace 'any' with your actual data type (e.g., User[] or StateData[])
  fieldset: any;
  fieldsSelected: any;
}

export interface PageComponentProps extends WithRouterProps {}
// Define expected state base interface
export interface PageComponentState {
  Data: any[];
  Fieldset: { [key: string]: any };
  loading?: boolean;
  error?: any;
}

export default class PageComponent<
P = PageComponentProps, 
S = PageComponentState
> extends React.Component<P, S> {
  protected uuid: string;
  protected isPage: boolean = false;
  protected requiresAuth: boolean = false;
  protected title: string = "";
  protected href: string = "";
  static condition: () => boolean = () => true;

  constructor(props: P) {
    super(props);

    this.uuid = PageComponent.guid();

    this.state = {
      Data: [],
      Fieldset: [],
      loading: true,
    } as S;
  }

  async componentDidMount() {
    if (this.href) {
      await this.fetchDefaultApiData();
    }
  }

  protected async fetchDefaultApiData() {
    if (!this.href) return;

    const stateUpdate: Partial<PageComponentState> = {
      loading: false,
    };

    try {
      const response = await axios.get(Config.serverURL + this.href);

      if (response.data?.Data !== undefined) {
        stateUpdate.Data = response.data.Data;
      }
      if (response.data?.Fieldset !== undefined) {
        stateUpdate.Fieldset = response.data.Fieldset;
      }

      this.setState(stateUpdate as Pick<S, keyof S>);
    } catch (error) {
      console.error(`Error loading API for ${this.constructor.name}:`, error);
      this.setState(stateUpdate as Pick<S, keyof S>);
    }
  }

  public getTitle(): string {
    return this.title;
  }

  public getHref(): string {
    return this.href;
  }

  public getUUID(): string {
    return this.uuid;
  }

  public isPageComponent(): boolean {
    return this.isPage;
  }

  public requiresAuthentication(): boolean {
    return this.requiresAuth;
  }

  public static getCondition(): () => boolean {
    return this.condition;
  }

  public static setCondition(condition: () => boolean): void {
    this.condition = condition;
  }

  static guid(): string {
    return Functional.guid();
  }
}