import React, { Component } from "react";
import Functional from "./functional";

interface PageComponentProps {}
interface PageComponentState {}

export default class PageComponent<P = PageComponentProps, S = PageComponentState> extends React.Component<P, S> {
  uuid: string;
  static isPage: boolean = false;
  static title: string = "";
  static href: string = "";
  static condition: () => boolean = () => true;

  public myComponent: boolean = true;

  constructor(props: P) {
    super(props);

    this.uuid = Functional.guid();
  }

  static guid(): string {
    return Functional.guid();
  }
}