## CHAPTER 01

## Software Requirements

## ACRONYMS

| ATDD   | Acceptance Test Driven Development           |
|--------|----------------------------------------------|
| BDD    | Behavior Driven Development                  |
| CIA    | Confidentiality, Integrity, and Availability |
| FSM    | Functional Size Measurement                  |
| INCOSE | International Council on Systems Engineering |
| JAD    | Joint Application Development                |
| JRP    | Joint Requirements Planning                  |
| SME    | Subject Matter Expert                        |
| SysML  | Systems Modeling Language                    |
| TDD    | Test Driven Development                      |
| UML    | Unified Modeling Language                    |

## INTRODUCTION

Software requirements should be viewed from two perspectives. The first is as an expression of the needs and constraints on a software product or project that contribute to the solution of a real-world problem. The second is that of the activities necessary to develop and maintain the requirements for a software product and for the project that constructs it. Both perspectives are presented in this knowledge area (KA).

If a team does a poor job of determining the requirements, the project, the product or both are likely to suffer from added costs, delays, cancellations and defects. One reason is that each software product requirement generally leads to many design decisions. Each design decision generally leads to many code-level decisions. Each decision can involve several test decisions, as well. In other words, determining the requirements correctly is highstakes work. If not detected and repaired early, missing, misinterpreted and incorrect requirements can induce exponentially cascading rework to correct them.

Real-world software projects tend to suffer from two primary requirements-related problems:

1. incompleteness: stakeholder requirements, and necessary detail, exist that are not revealed and communicated to the software engineers;
2. ambiguity: requirements are communicated in a way that is open to multiple interpretations, with only one possible interpretation being correct.

Beyond the obvious short-term role requirements play in initial software construction, they also play a less recognized but still important role in long-term maintenance. Upon receiving software without any supporting documentation, a software engineer has several means to determine what that code does, such as execute it, step through it with a debugger, hand-execute it, statically analyze it, and so on. The challenge is determining what that code is intended to do. What is generally referred to as a bug — but is better called a defect — is simply an observable difference between what the software is intended to do and what it does. The role of requirements documentation throughout the service life of the software is to capture and communicate intent for software engineers who maintain the code but might not have been its original authors.

The Software Requirements KA concerns developing software requirements and managing those requirements over the software's service life. This KA provides an understanding that software requirements:

are not necessarily a discrete front-end activity of the software development life This KA is also related, but somewhat less so, cycle but rather a process initiated at a to the Software Configuration Management, project's beginning that often continues Software Engineering Management and to be refined throughout the software's Software Quality KAs. Software Configuration entire service life; Management approaches can be applied to need to be tailored to the organization trace and manage requirements; software and project context. quality looks at how well formed the requirements are, and engineering management can The term requirements engineering is often use the status of requirements to evaluate the

The whats and hows of software requirements work on a project should be determined by the nature of the software constructed, not by the life cycle under which it is constructed. Insofar as requirements documentation captures and communicates the software's intent, downstream maintainers should not be able to discern the life cycle used in earlier development from the form of those requirements alone.

used to denote the systematic handling of completion of the project. requirements. For consistency, the term engineering will not be used in this KA other than for software engineering per se.

The Software Requirements KA is most closely related to the Software Architecture, Software Design, Software Construction, Software Testing, and Software Maintenance KAs, as well as to the models topic in the Software Engineering Models and Methods KA, in that there can be high value in specifying requirements in model form.

## BREAKDOWN OFTOPICS FOR SOFTWARE REQUIREMENTS

The topic breakdown for the Software Requirements KA is shown in Figure 1.1.

## 1. Software Requirements Fundamentals

This KA is also related to the Software Life Cycles topic in the Software Engineering Process KA, in that this KA's focus is on what and how requirements work can and should be done, whereas the project's life cycle determines when that work is done. For example, in a waterfall life cycle, all requirements work is essentially done in a discrete Requirements phase and is expected to be substantially complete before any architecture, design and construction work occurs in subsequent phases. Under some iterative life cycles, initial, highlevel requirements work is done during an Inception phase, and further detailing is done during one or more Elaboration phases. In an Agile life cycle, requirements work is done element of functionality is constructed.

<!-- formula-not-decoded -->

Formally, a software requirement has been defined as [28]:

- a condition or capability needed by a user to solve a problem or achieve an objective;
- a condition or capability that must be met or possessed by a system or system component to satisfy a contract, standard, specification or other formally imposed document;
- a documented representation or capability as in (1) or (2) above.

This formal definition is extended in this incrementally, just in time, as each additional KA to include expressions of a software project's needs and constraints.

Figure 1.1. Breakdown of Topics for the Software Requirements KA

<!-- image -->

At its most basic, a software requirement relationships among those categories. (See

is a property that must be exhibited to solve a also [5, c1] [6, c1] [9, c4].) Each category is real-world problem. It might aim to automate further described below. all or part of a task supporting an organization's business policies and processes, correct 1.3. Software Product Requirements and existing software's shortcomings, or control a Software Project Requirements device — just a few of the many problems for [1*, c1pp14-15] which software solutions are possible.

Business policies and processes, as well as Software product requirements specify the device functions, are often very complex. By software's expected form, fit or function. extension, software requirements are often a Software project requirements — also called complex combination of requirements from process requirements or, sometimes business various stakeholders at different organizarequirements — constrain the project that tional levels who are involved or connected constructs the software. Project requirewith some aspect of the environment in which ments often constrain cost, schedule and/or the software will operate. staffing but can also constrain other aspects Clients, customers and users usually impose of a software project, such as testing envirequirements. However, other third parties, ronments, data migration, user training, like regulatory authorities and, in some cases, and maintenance. Software project requirethe software organization or the project itself, ments can be captured in a project charter might also impose requirements. (See also [5, or other high-level project initiation docc1] [6, c1] [9, c4].) ument. They are most relevant to how the project is managed (see the Software 1.2. Categories of Software Requirements Engineering Management KA) or what [1*, c1pp7-12] [2*, s4.1] life cycle process should be used (see the Software Engineering Process KA). This Fisore os os dos  s os  s  ds  osrct

requirements defined in this KA and the requirements further.

Figure 1.2. Categories of Software Requirements

<!-- image -->

## 1.4. Functional Requirements

<!-- formula-not-decoded -->

implementation: What computing platform(s)? What database engine(s)? How accurate do results need to be? How quickly must results be presented? How many records of a certain type need to be stored? Some nonfunctional requirements might relate to the operation of the software. (See the Operation and Maintenance KA.) (See also [5, c1] [6, c11] [9, c4].)

The nonfunctional requirements can be further divided into technology constraints and quality of service constraints. They have essential relationships among themselves, which affect them positively or negatively and require that, whenever a nonfunctional requirement is modified, the impact it may cause on others should be considered.

## 1.6. Technology Constraints

Functional requirements specify observable behaviors that the software is to provide — policies to be enforced and processes to be carried out. Example policies in banking softwan asct s a ,  a ae au oer our o oe o o  the balance of an account shall never be negative." Example processes could specify the meanings of depositing money into an account, withdrawing money from an account and transferring money from one account to another.

Even highly technical (nonbusiness-oriented) software, such as software that implements the transmission control protocol/ internet protocol (TCP/IP) network communications protocol, has policies and processes: "a Port shall be able to exist with zero, one, or many associated Connections, but a Connection shall exist on exactly one associated Port," "acceptable states of a Connection shall be listen,''syn sent,'established,' 'closing, . . . , and "if the time-to-live of a Segment reaches zero, that Segment shall be deleted." (See [5, c1] [6, c10] [9, c4].)

These requirements mandate — or prohibit — use of specific, named automation technologies or defined infrastructures. Examples are requirements to use specific computing platforms (e.g., Windows™M, macOS™M, Android OS™, iOS™), programming languages (e.g., Java, C++, C#, Python), compatibility with specific web browsers (e.g., Chrome™™, Safari™, Edge), given database engines (e.g., Oracle™, SQL Server™, MySQL™), and general technologies (e.g., reduced instruction set computer (RISC), Relational Database). A requirement prohibiting use of pointers would be another example. (See also [9, c4].)

## 1.7. Quality of Service Constraints

These requirements do not constrain the use of specific, named technologies. Instead, these specify acceptable performance levels an automated solution must exhibit. Examples are response time, throughput, accuracy, reliability and scalability. ISO/IEC 25010: "System and software engineering – Systems 1.5. Nonfunctional Requirements and software Quality Requirements and [1*, c1pp10-11] [2*, s4.1.2] Evaluation (SQuaRE) – System and software quality models" [27] contains a large list of Nonfunctional requirements in some way conthe kinds of quality characteristics that can be strain the technologies to be used in the relevant for software. (See also [9, c4].) Safety and security are also a particularly important The Perfect Technology Filter originally topic where requirements tend to be overdescribed in [18, c1-4] but also explained in looked. (See the Security KA for details on [8] and [9, c4] helps separate functional from the kinds of specific security requirements nonfunctional requirements. Simply put, that should be considered.) (See also [2*, c13].) functional requirements are those that would still need to be stated even if a computer with 1.8. Why Categorize Requirements This Way? infinite speed, unlimited memory, zero cost, no failures, etc., existed on which to construct Categorizing requirements this way is useful the software. All other software product for the following reasons: requirements are constraints on automation technologies and are therefore nonfunctional.

requirements in one category tend to Large systems often span more than one coi sy     nes  o  ss   oned categories; in [9, c6], recursive design shows how nonelicitation techniques often vary functional requirements in a parent domain by source; can become, or can induce, functional requireanalysis techniques vary by category; ments in a child domain. For example, a non· specification techniques vary by category; functional requirement about user security · validation authorities vary by category; in a parent banking domain can become or · the different categories affect the resulting can induce functional requirements in a child software in different ways. security domain. Similarly, cross-cutting nonfunctional requirements about auditing and In addition, organizing the requirements transaction management in a parent banking in these categories is beneficial in the foldomain can become or induce functional lowing ways: requirements in a child auditing domain and a child transaction domain. Decomposing large systems into a set of related domains significantly reduces complexity.

The International Council on Systems Engineering (INCOSE) defines a system as "an interacting combination of elements to accomplish a defined objective. These include lated; stakeholders, not software engi- hardware, software, firmware, people, inforneers, are the experts in the policies and mation, techniques, facilities, services, and

- complexity can be better managed because different areas can be addressed separately; software engineers can deal with policy and process complexities without 1.9. System Requirements and Software worrying about automation technology Requirements issues at the same time (and vice versa). One large problem becomes two smaller ones. This is classic divide and conquer complexity management;
- distinct areas of expertise can be isoprocesses to be automated. Software other support elements" [24]. engineers, not stakeholders, are the In some cases, it is either useful or mandatechnology experts. When a business tory to distinguish system requirements from expert is given interspersed functional software requirements. System requirements and nonfunctional requirements for apply to larger systems — for example, an review or validation, they might give autonomous vehicle. Software requirements up because they don't understand — or apply only to an element of software in that even care about — the technology issues. larger system. Some software requirements The relevant requirements reviewer can may be derived from system requirements. focus on just the subset of requirements (See also [5, c1].) In other cases, the software is relevant to them. itself the system of interest, and hardware and

Figure 1.3. Software Requirements Activities Figure 1.3. Software Requirements Activities

<!-- image -->

support system are regarded as the platform or infrastructure, so that the system requirements are mostly software requirements.

## 1.10. Derived Requirements

In practice, requirements can be context-sensitive and can depend on perspective. An external stakeholder can impose a scope requirement, and this would be a requirement for the entire project — even if that project involves hundreds of software engineers. An architect's also [5, c2-3] [6, c3-7].) decision to use a pipes-and-filters architecture style would not be a requirement from the perspective of the overall project stakeholders, but a design decision. But that same decision, when seen from the perspective of a sub-team responsible for constructing a particular filter, would be considered a requirement.

Requirements development, as a whole, can be thought of as "reaching an agreement on what software is to be constructed." In contrast, requirements management can be considered "maintaining that agreement over time." Each activity is presented in this KA. Requirements development activities are presented as separate topics, with requirements management presented as a single topic. (See also [5, c1] [6, 2].)

## 2. Requirements Elicitation

<!-- formula-not-decoded -->

The goal of requirements elicitation is to surface candidate requirements. It is also called requirements capture, requirements discovery or requirements acquisition. As stated earlier, one problem in requirements work on real-world software projects is incompleteness. This can be the result of inadequate elicitation. Although there is no guarantee that a set of requirements is complete, well-executed elicitation helps minimize incompleteness. (See

<!-- formula-not-decoded -->

Requirements come — can be elicited — from many different sources. All potential requirements sources should be identified and evaluated. A stakeholder can be defined as any person, group or organization that:

- clients — those who pay for the software to be constructed (e.g., organizational management);

The aerospace industry has long used the term derived requirement to mean a requirement that was not made by a stakeholder external to the overall project but that was · is actively involved in the project; imposed inside the larger development team. · is affected by the project's outcome; The architect's pipes-and-filters decision fits can influence the project's outcome. this definition. That choice would be seen as a design decision from the point of view of Typical stakeholders for software projects external stakeholders, but as a requirement for include but are not limited to the following: the sub-teams responsible for developing each filter. (See also [9, c4].)

<!-- formula-not-decoded -->

Figure 1.3 shows the requirements development and management activities.

- customers — those who decide whether a software product will be put into service;
- users — those who interact directly or indirectly with the software; users can

often be further broken down into dis- techniques work better with certain staketinct user classes that vary in frequency holder classes than others. Common stakeof use, tasks performed, skill and knowlholder elicitation techniques include the edge level, privilege level, and so on; following:

- subject matter experts (SMEs);
- operations staff;
- interviews;
- first-level product support staff;
- relevant professional bodies;
- regulatory agencies;
- special interest groups;
- people who can be negatively affected if the project is successful;
- developers.
- meetings, possibly including brainstorming;
- joint application development (JAD) [13], joint requirements planning (JRP) [14] and other facilitated workshops;
- protocol analysis;
- focus groups;

Stakeholder classes are groups of stakeholders that have similar perspectives and needs. Working on a software project in terms of stakeholder classes rather than with individual stakeholders can produce important, additional insight.

- questionnaires and market surveys;
- exploratory prototyping, including low-fidelity and high-fidelity user interface prototyping [1*, c15];
- user story mapping.

Elicitation can be difficult, and the software Many projects benefit from performing engineer needs to know that (for example) users a stakeholder analysis to identify as many might have difficulty describing their tasks, important stakeholder classes as possible. This leave important information unstated or be reduces the possibility that the requirements unwilling or unable to cooperate. Elicitation are biased toward better-represented stakeis not a passive activity. Even if cooperative holders and away from less well-represented and articulate stakeholders are available, the stakeholders. The stakeholder analysis can software engineer must work hard to elicit also inform negotiation and conflict resoluthe right information. Many product requiretion when requirements from one stakeholder ments are tacit or can be found only in inforclass conflict with requirements from another. mation that has yet to be collected. (See also [5, c3] [6, c3].)

Requirements can also be elicited from Requirements are not limited to only sources other than stakeholders. Such sources

coming from people. Other, non-person and techniques include the following: requirements sources can include:

- documentation such as requirements for previous versions, mission statements, concept of operations;
- other systems;
- larger business context including organizational policies and processes;
- computing environment.

## 2.2. Common Requirements Elicitation Techniques [1*, c7] [2*, s4.3] 2.2. Common Requirements Elicitation Techniques [1*, c7] [2*, s4.3]

A wide variety of techniques can be used to elicit requirements from stakeholders. Some A wide variety of techniques can be used to elicit requirements from stakeholders. Some

- previous versions of the system;
- defect tracking database for previous versions of the system;
- systems that interface with the system under development;
- competitive benchmarking;
- literature search;
- quality function deployment (QFD)'s House of Quality [15];
- observation, where the software engineer studies the work and the environment where the work is being done;
- apprenticing, where the software engineer learns by doing the work;
- usage scenario descriptions;

- decomposition (e.g., capabilities into epics into features into stories);

The overall collection of requirements should be:

- task analysis [16];
- design thinking (empathize, define, ideate, prototype, test) [17];
- ISO/IEC 25010: "System and software engineering - Systems and software Quality Requirements and Evaluation (SQuaRE) – System and software quality models" [27];
- security requirements, as discussed in the Security KA;
- complete — The requirements adequately address boundary conditions, exception conditions and security needs;
- concise — No extraneous content in the requirements
- internally consistent — No requirement conflicts with any other;
- externally consistent — No requirement conflicts with any source material;
- applicable standards and regulations.

(See also [5, c3] [6, c4-7].)

- feasible — A viable, cost-effective solution can be created within cost, schedule, staffing, and other constraints.

3. Requirements Analysis [1*, c8-9] In some cases, an elicited statement represents a solution to be implemented rather Requirements are unlikely to be elicited in than the true problem to be solved. This their final form. Further investigation is usu- risks implementing a suboptimal solution. ally needed to reveal the full, true requireThe 5-whys technique (e.g., [3*, c4]) involves ments suggested by the originally elicited repeatedly asking, "Why is this the requireinformation. Requirements analysis helps ment?" to converge on the true problem. software developers understand the meaning Repetition stops when the answer is, "If that and implications of candidate requirements, isn't done, then the stakeholder's problem has both individually and in the context of the not been solved." Often, the true problem is overall set of requirements. reached in two or three cycles, but the technique is called 5-whys to incentivize engineers 3.1. Basic Requirements Analysis to push it as far as possible.

## 3.2. Economics of Quality of Service Constraints [3*]

Quality of service constraints can be particengineers do not consider them from an economic perspective [9, c4]. Figure 1.4 illusrn   e  s  e n unoe only one way); quality of service constraint, such as capacity, i   b) compliance or noncompliance can be increases with performance level. This curve is clearly demonstrated; mirrored vertically for quality of service conbe binding, meaning that clients are straints whose value decreases as performance willing to pay for it and unwilling not level increases (response time and mean time to have it; to repair would be examples).

<!-- formula-not-decoded -->

The following list of desirable properties of requirements can guide basic requirements analysis. The software engineer seeks to establish any of these properties that do not ularly challenging. This is generally because hold yet. Each requirement should:

· atomic, represent a single decision Over the relevant range of performance · represent true, actual stakeholder needs; levels, the stakeholders have a corresponding · use stakeholder vocabulary; value if the system performs at that level. The · be acceptable to all stakeholders. value curve has two important points:

1. Perfection point — This is the most favorable level of performance, beyond which there is no additional benefit. Even if the system can perform better than the perfection point, the customer cannot use that capacity. For example, a social media system that supports more members than the world population would have this excess capacity.
2. Fail point — This is the least favorable level of performance, beyond which there is no further reduction in benefit. For example, the social media system might need to support at least a minimum market share to be viable as a platform.

<!-- image -->

A quantified requirement point, even if stated explicitly, is usually arbitrary. It is often based on what a client feels justified requesting, given what they are paying for the software. Even if the software engineers cannot construct a system that fully achieves the stated requirement point, the software typically still has value; it just has less value than the client expected. Further, the ability to exceed the requirement point can significantly increase value in some cases.

Figure 1.4. Value as a Function of Level of Performance

$

Figure 1.5. Most Cost-Effective Level of Performance

<!-- image -->

level. For example, the more modifiable code is, the more reliable it tends to be, as both modifiability and reliability are, to a degree, a consequence of how clean the code is. On the other hand, the higher the code's speed, the less modifiable it might be, because high speed is often achieved through optimizations that make the code more complex.

The cost to achieve a given performance level is usually a step function. First, for a given investment level, there is some maximum achievable performance level. Then, additional investment is needed, and that further investment enables performance up to a new, more favorable maximum. Figure 1.5 illustrates the most cost-effective performance level — the performance level with the maximum positive difference between the value at that performance level and the cost to achieve it.

(See the Software Engineering Economics 3.3. Formal Analysis KA or [3*] for more information on performing economic analyses such as this.)

The software engineer should pay particular attention to positive and negative relationships between quality of service constraints (e.g., Figure 14-1 in [1*, c14]). Some quality of service constraints are mutually supporting; improving one's performance level will automatically improve the other's performance

[2*, s12.3.2-12.3.3]

Formal analysis has shown benefits in some application domains, particularly high-integrity systems (e.g., [5, c6]). The formal expression of requirements depends on the use of a specification language with formally defined semantics. Formality has two benefits. First, formal requirements are precise and concise, which (in principle) will reduce the possibility The software engineer can focus on underfor misinterpretation. Second, formal requirestanding the range of variations needed to ments can be reasoned over, permitting satisfy all stakeholders. The software can be desired properties of the specified software to designed using design to invariants to accombe proved. This permits static validation that modate the invariant requirements and design the software specified by the requirements for change to incorporate customization points does have the properties (e.g., absence of to configure an instance of the system to best deadlock) that the customer, users and soft- fit relevant stakeholders. In a simple example, ware engineer expect it to have. some users of a weather application require This topic is related to Formal Methods temperatures displayed in degrees Celsius in the Software Engineering Models and while others require degrees Fahrenheit. Methods KA.

## 4. Requirements Specification

## 3.4. Addressing Conflict in Requirements

[1*, c10-14, c20-26] [2*, s4.4, c5]

When a project has more — and more diverse Requirements specification concerns recording — stakeholders, conflicts among the require- the requirements so they can be both rememments are more likely. One particularly bered and communicated. Requirements important aspect of requirements analysis specification might be the most contentious is identifying and managing such conflicts topic in this KA. Debate centers on ques(e.g., [6, c17]). Once conflicting requirements tions such as: have been identified, the engineer may consider two different approaches to managing · should requirements be written that conflict (and possibly other approaches down at all? as well) and determine the most appropriate if requirements are written down, what course of action. form should they take?

- if requirements are written down, should they also be maintained over time?

There are no standard answers to these questions; the answer to each can depend on

- the software engineer's familiarity with the business domain;
- precedent for this kind of software;
- degree of risk (e.g., probability, severity) of incorrect requirements;
- staff turnover anticipated during the service life of the software;
- geographic distribution of the development team members;

One approach is to negotiate a resolution among the conflicting stakeholders. In most cases, it is unwise for the software engineer to make a unilateral decision, so it becomes necessary to consult with the stakeholders to reach a consensus resolution. It is often also factors such as the following: important, for contractual reasons, that such decisions be traceable back to the customer. A specific example is project scope management — namely, balancing what's desired in the stated software product requirements with what can be accomplished given the project requirements of cost, schedule, staffing and other project-level constraints. There are many useful sources for information on negotiation and conflict resolution [25].

Another approach is to apply product family development (e.g., [20]). This involves separating requirements into two categories. The first category contains the invariant requirements. These are requirements that all stakeholders agree on. The second category contains the variant requirements, where conflict exists.

- stakeholder involvement over the course of the project;
- whether the use of a third-party service, packaged solution or open source library is anticipated;
- whether any design or construction will be outsourced;

t   d  d    n testing expected; information be packaged and presented so · effort needed to use a candidate specificathat each consumer can get the information tion technique; they need with the least effort?

- accuracy needed from the requirements-based estimates;
- extent of requirements tracing necessary, if any;
- contractual impositions of requirements specification content and format.

There is a degree of overlap and dependency between requirements analysis and specification. Use of certain requirements specification techniques — particularly model-based requirements specifications — permit and encourage requirements analysis that can go beyond what has already been presented.

As stated in this KA's introduction, the Documented software requirements should whats and hows of software requirements be subject to the same configuration manwork on a project should be determined by agement practices as the other deliverables the nature of the software constructed, not of the software life cycle processes. (See the by the life cycle under which it is constructed. Configuration Management KA for a detailed Downstream maintainers should not be able discussion.) In addition, when practical, the to discern the life cycle used in earlier develindividual requirements are also subject to opment from the form of those requirements configuration management and traceability, alone. The chosen life cycle's effect should be which is generally supported by a requirements limited to the completeness of the requiremanagement tool. (See Topic 8, Software ments at any point in the project. Under a Requirements Tools.) waterfall life cycle, the requirements are expected to be completely specified at the end requirements specification techniques, each of the Requirements phase. Under an Agile of which is discussed below. The requirements life cycle, the requirements are expected to specification for a given project may also use change, grow, or be eliminated continuously various techniques. ISO/IEC/IEEE 29148 and not be complete until the project's end.

There are several general categories of [26], as well as [1*, c10-14], [5, c4], [6, c16], and many others offer templates for requirements documentation.

## 4.1. Unstructured Natural Language Requirements Specification

[1*, c11] [2*, s4.4.1]

Natural language requirements specifications express requirements in common, ordinary language. Natural language requirements specifications can be unstructured or structured.

A typical unstructured natural language requirements specification is a collection of statements in natural language, such as, "The system shall . . . ." For example, business rules are statements that define or constrain some The most basic recommendation for aspect of the structure or the behavior of the requirements documentation is to base deci- business to be automated. "A student cannot sions on an audience analysis. Who are the register in next semester's courses if there different consumers who will need informa- remain any unpaid tuition fees" is an example tion from a requirements specification? What of a business rule that serves as a requirement

Some organizations have a culture of documenting requirements; some do not. Dynamic startup projects are often driven by a strong product vision and limited resources; their teams might view requirements documentation as unnecessary overhead. But as these products evolve and mature, software engineers often recognize that they need to recover the requirements that motivated product features in order to assess the impact of proposed changes. Hence, requirements documentation and change management become important to long-term success. A project's approach to requirements in general, and to requirements specification in particular, may evolve over the service life of that software.

Figure 1.6. Example of Structured Natural Language Specification for a Single Use Case

| Use case #66          | Use case name: Reserve flight(s)                                                                                                                                                                                        |
|-----------------------|-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| Triggering event(s)   | Customer requests reservation(s) on flight(s)                                                                                                                                                                           |
| Parameters            | Passenger, itinerary, fare class, payment method(s)                                                                                                                                                                     |
| Requires              | Legal itinerary, fare class restrictions met                                                                                                                                                                            |
| Guarantees            | Seat(s) reserved for passenger on itinerary flight(s)                                                                                                                                                                   |
| Normal course         | Non-FF passenger, all domestic itinerary, Economy fare class, credit/debit card                                                                                                                                         |
| Alternative course(s) | Is FF passenger: [None, Silver, Gold, Platinum, Elite] Itinerary: [all international, mixed domestic + international] Fare class: [Basic economy, Premium Economy, Business, First] Payment method: [Voucher, FF miles] |
| Exceptions            | C/D card declined, voucher doesn't exist, voucher expired, FF account doesn't exist, insufficient miles in FF account                                                                                                   |

for a university's course-registration software. [11] for guidelines on writing good use case

Some projects can publish a user manual as specifications.) a satisfactory requirements specification, although there are limits to how effective this can be. (See also [5, c4] [26].)

- 4.2. Structured Natural Language Requirements Specification

Structured natural language requirements specifications impose constraints on how the This general approach includes two specific -    o:  s a  a o sos increase precision and conciseness. ment (ATDD) and behavior driven developThe user story format, "As a &lt;role&gt; I want &lt;capability&gt; so that &lt;benefit&gt;" as well as decision tables are other examples. (See also [5, c4] [6, c12, c16] [7, c2-5].)

## [1*, c8] [2*, s4.4.2]4.3. Acceptance Criteria-Based Requirements Specification

The simplest example might be the ment (BDD). actor-action format. The actor is the entity ATDD [2*, s3.2.3, s8.2] is a part of the larger responsible for carrying out the action, and test driven development (TDD) approach. aet  ( S e   S  a    e ain gering event might precede the actor, and idea of TDD is that test cases precede conthe action might be followed by an optional struction. Therefore, no new production code condition or qualification. The statement is written and no existing code is modified "When an order is shipped, the system shall unless at least one test case fails, either at the create an Invoice unless the Order Terms unit test level or at the acceptance test level. are Prepaid"" uses actor-action format. The ATDD process has three steps: The triggering event is "When an order is shipped." The actor is "the system." The 1. A unit of functionality (e.g., a user story) action is "create an Invoice." The condition/ is selected for implementation. qualification is "unless the Order Terms are 2. One or more software engineers, one or 'Prepaid'."

Another example is a use case specification template, as shown in Figure 1.6. (See

- more business domain experts, and possibly one or more QA/test professionals meet — before any production design or

construction work is done — to agree on &lt;stimulus&gt; then &lt;outcome&gt; [and &lt;possibly more outcomes&gt;]."

to withdraw cash from the automated teller machine (ATM) so that I can get money must fail on the existing software. The without going to the bank," one scenario could

a set of test cases that must pass to show that the unit of functionality has been correctly implemented.

3. At least one of those acceptance test cases existence of at least one failing test case gives the software engineer(s) permission to create or modify production code to pass all of the agreed-upon test cases. This step might require several iterations. The code may also be refactored during this step.

When all acceptance test cases have passed, and presumably all unit and integration test cases as well, then the unit of functionality is deemed to have been completely and correctly implemented. The ATDD process returns to step 1, where a new unit of functionality is selected, and the cycle repeats.

ATDD might seem to be a testing technique rather than a requirements specification technique. On the other hand, a test case has the general form of "When given input that looks like X, we expect the software to produce results that look like Y." The key is the underlined phrase, "we expect the software to produce." If we simply modify that phrase to say, "the software shall produce," as in "When given input that looks like X, the software shall produce results that look like Y," what first looked like a test case now looks like a requirement. Technically, one acceptance test case can encompass more than one single requirement, but the general idea holds that the ATDD test cases are essentially precise, unambiguous statements of requirements.

The BDD approach [19] is slightly more structured, and business domain experts typically prefer it over ATDD because it is less technical in appearance. In BDD, the unit of functionality is described as a user story, in a form such as this: "As a &lt;role&gt; I want &lt;capability&gt; so that &lt;benefit&gt;." This leads to the identification and specification of a set of "scenarios" in this form: "Given &lt;some context&gt; [and &lt;possibly more context&gt;], when If the story is "As a bank customer, I want be that "the account has a sufficient balance." This scenario could be detailed as "Given the account balance is $500, and the customer's bank card is valid, and the automated teller machine contains enough money in its cash box, when the Account Holder requests $100, then the ATM should dispense $100 and the account balance should be $400, and the customer's bank card should be returned."

Another scenario could be that "the account has an insufficient balance" and could be detailed as "Given the account balance is $50, and the customer's bank card is valid, and the automated teller machine contains enough money in its cash box, when the Account Holder requests $100, then the ATM should not dispense any money, and the ATM should say there is an insufficient balance, the balance should remain at $50, and the customer's bank card should be returned."

The goal of BDD is to have a comprehensive set of scenarios for each unit of functionality. In the withdrawing cash situation, additional scenarios for "The Bank Customer's bank card has been disabled" and "The ATM does not contain enough money in its cash box" would be necessary.

The acceptance test cases are obvious from the BDD scenarios.

Acceptance criteria-based requirements specification directly addresses the requirements ambiguity problem. Natural languages are inherently ambiguous, but test case language is not. In acceptance-based criteria requirements specification, the requirements are written using test case language, which is very precise. On the other hand, this does not inherently solve the incompleteness problem. However, combining ATDD or BDD with appropriate functional test coverage criteria, such as Domain Testing, Boundary Value Analysis and Pairwise Testing (see the Software Testing KA), can reduce the

- likelihood of requirements incompleteness. 2. Semiformal modeling, for example [9, (See also [9, c1, c12].) c6-12], provides a definition of the modeling language semantics ([9, Appendix 4.4. Model-Based Requirements Specification L]), but that definition has not been [1*, c12] [2*, c5] [4*] formally proved to be complete and consistent.

Another approach to avoiding the inherent 3. Formal modeling, for example, Z, the ambiguity of natural languages is to use modVienna development method (VDM), eling languages such as selected elements of specification and description language the unified modeling language™ (UML) or (SDL) and [5, c7] have very precisely systems modeling language™ (SysML). Much defined semantics that allow specificalike the blueprints used in building constructions to be mechanically analyzed for the tion, these modeling languages can be used presence or absence of specific properties in a computing technology-free manner to to help avoid critical reasoning errors. precisely and concisely specify functional The term correctness by construction has requirements [9, c1-2]. This topic is closely been used for development in this conrelated to the Software Engineering Models text. (See the Formal Methods section in and Methods KA. Requirements models fall the Software Engineering Models and into two general categories: Methods KA.)

Generally, the more formal a requirements model is, the less ambiguous it is, so software engineers are less likely to misinterpret the requirements. More formal requirements models can also be:

1. Structural models for specifying policies to be enforced: These are logical class models as described in, for example, [9, c8]. They are also called conceptual data models, logical data models and entity-relationship diagrams.
2. more concise and compact;
3. easier to translate into code, possibly mechanically;
4. used as a basis for deriving acceptance test cases.

One important message in [4*] is that while eling, as described in [1*, c12-13], [8], formal modeling languages are stronger than semiformal and Agile modeling, formal notations can burden both the model creator and Model-based requirements specifica- human readers. Wing's compromise is to use tions vary in the degree of model formality. formally defined underpinnings (e.g., in Z) Consider the following: for surface syntaxes that are easier to read and write (e.g., UML statecharts).

2. Behavioral models for specifying processes to be carried out: These models include use case modeling as described in [9, c7], interaction diagrams as described in [9, c9] and state modeling as described in [9, c10]. Other examples are UML activity diagrams and data-flow mod[10] and [18].
1. Agile modeling (see, for example, [10]) is the least formal. Agile models can be little more than rough sketches whose goal is to communicate important information rather than demonstrate proper use of modeling notations. In this type of modeling, the effect of the communication is considered more important than the form of the communication.

<!-- formula-not-decoded -->

Over and above the basic requirements statements already described, documenting additional attributes for some or all requirements can be useful. This supplemental detail can help software engineers better interpret and manage the requirements [6, the previous version. An advantage of this c16]. Possible additional attributes include approach is that a reader can understand all the following: requirements in a single document instead of having to keep track of cumulative additions, · tag to support requirements tracing; modifications and deletions across a series of

- description (additional details about the specifications. requirement);

Some organizations combine these two rationale (why the requirement is approaches, producing intermediate releases important); (e.g., x.1, x.2 and x.3) that are specified incresource (role or name of the stakeholder mentally and major releases (e.g., 1.0, 2.0 and who imposed this requirement); 3.0) that are specified comprehensively. The use case or relevant triggering event; nnea nuer r   ndr ndr necr type (classification or category of the than the requirements specifications for the requirement — e.g., functional, quality last major release to obtain the complete set of service); of specifications.

- dependencies;
- conflicts;

## 5. Requirements Validation

- acceptance criteria;

<!-- formula-not-decoded -->

- priority (see Requirements Prioritization later in this KA);

Requirements validation concerns gaining stability (see Requirements Stability and confidence that the requirements represent Volatility later in this KA); the stakeholders' true needs as they are curwhether the requirement is common or a rently understood (and possibly documented).

- variant for product family development Key questions include the following: (e.g., [20]);
- supporting materials;
- the requirement's change history.

Gilb's Planguage (short for Planning Language) [7] recommends attributes such as scale, meter, minimum, target, outstanding, past, trend and record.

## 4.6. Incremental and Comprehensive Requirements Specification

- do these represent all requirements relevant at this time?
- are any stated requirements not representative of stakeholder needs?
- are these requirements appropriately stated?
- are the requirements understandable, consistent and complete?
- does the requirements documentation conform to relevant standards?

Projects that explicitly document requireThree methods for requirements validation ments take one of two approaches. One can tend to be used: requirements reviews, simbe called incremental specification. In this ulation and execution, and prototyping. (See approach, a version of the requirements speci- also [5, c5] [6, c17] [9, c12].) fication contains only the differences — additions, modifications and deletions — from 5.1. Requirements Reviews the previous version. An advantage of this [1*, c17pp332-342] [2*, c4p130] approach is that it can produce a smaller volume of written specifications.

The most common way to validate is by The other approach can be called compre- reviewing or inspecting a requirements docuhensive specification. In this approach, each ment. One or more reviewers are asked to look sn nns sn so ons on rons nn sons tains all requirements, not just changes from lack of clarity and deviation from accepted practice. Review from multiple perspectives Prototypes can help expose software engiis preferred: neers' assumptions and, where needed, give useful feedback on why they are wrong. For · clients, customers and users check that example, a user interface's dynamic behavior their wants and needs are completely and might be better understood through an aniaccurately represented; mated prototype than through textual other software engineers with expertise description or graphical models. However, a in requirements specification check that danger of prototyping is that cosmetic issues the document is clear and conforms to or quality problems with the prototype can applicable standards; distract the reviewers' attention from the core software engineers who will do architecunderlying functionality. Prototypes can also ture, design or construction of the soft- be costly to develop. However, if a prototype ware that satisfies these requirements helps engineers avoid the waste caused by check that the document is sufficient to trying to satisfy erroneous requirements, its support their work. cost can be more easily justified.

Providing checklists, quality criteria or 6. Requirements Management Activities a "definition of done" to the reviewers can [1*, c27-28] [2*, s4.6] guide them to focus on specific aspects of the requirements specification. (See Reviews and Requirements development, as a whole, can be Audits in the Software Quality KA.)

## 5.2. Simulation and Execution

Nontechnical stakeholders might not want to spend time reviewing a specification in detail. Some specifications can be subjected to simulation or actual execution in place of or in addition to human review. To the extent that the requirements are formally specified (e.g., in a model-based specification), software engineers can hand interpret that specification and "execute" the specification. Given a sufficient set of demonstration scenarios, stakeholders can be convinced that the specification defines their policies and processes completely and accurately. (See [9, c12].)

<!-- formula-not-decoded -->

thought of as "reaching an agreement on what software is to be constructed." (See Figure 1.3.) In contrast, requirements management can be thought of as "maintaining that agreement over time." This topic examines requirements management. (See also [5, c9].)

## 6.1. Requirements Scrubbing

The goal of requirements scrubbing [22, c14, c32] is to find the smallest set of simply stated requirements that will meet stakeholder needs. Doing so will reduce the size and complexity of the solution, thus minimizing the effort, cost and schedule to deliver it. Requirements scrubbing involves eliminating requirements that:

- are out of scope;
- would not yield an adequate return on investment;
- are not that important.

If the requirements specification is not in a form that allows direct simulation or exeAnother important part of the process cution, an alternative is to have a software is to simplify unnecessarily complicated engineer build a prototype that concretely requirements. demonstrates some important dimension of In waterfall and other plan-based life an implementation. This demonstrates the cycles, requirements scrubbing can be coorsoftware engineer's interpretation of those dinated with requirements reviews for validarequirements. tion; scrubbing should occur just before the validation review. In Agile life cycles, scrub- cost, schedule or staffing constraints on the only the highest-priority requirements are brought into a sprint (iteration).

Projects using waterfall or other plan-based life cycles should have an explicit requirements change control process that includes:

bing happens implicitly in iteration planning; project. When requirements scope exceeds the cost, schedule or staffing constraints, then either that scope must be reduced (presumably by removing a sufficient number of 6.2. Requirements Change Control the lowest-priority requirements), capacity [1*, c28] [2*, s4.6] must be increased (by extending the schedule or increasing the budget and/or staffing), or Change control is central to managing some appropriate combination thereof must be requirements. This topic is closely linked to the negotiated. Where possible, scope matching Software Configuration Management KA. should be quantitative instead of qualitative, i.e., in terms of functional size units.

In waterfall and other plan-based life cycles, scope matching can be coordinated with requirements validation; the scope matching · a means to request changes to previously should occur just before the validation review. agreed-upon requirements; In Agile life cycles, as long as some variant of an optional impact analysis stage to more velocity-based sprint planning is done, then the thoroughly examine benefits and costs of only work allowed into a sprint/iteration will a requested change; be the work that can reasonably be expected a responsible person or group who to be completed during that sprint/iteration.

- decides to accept, reject, or defer each requested change;
- a means to notify all affected stakeholders of that decision;
- a means to track accepted changes to closure.

## 7. Practical Considerations

<!-- formula-not-decoded -->

All stakeholders must understand and agree that accepting a change means accepting its impact on schedule, resources and/or commensurate change in scope elsewhere in the project. Ideally the change in scope should be objectively quantifiable, i.e., in terms of functional size units.

In contrast, requirements change management happens implicitly in Agile life cycles. In these life cycles, any request to change previously agreed-upon requirements becomes just another item on the product backlog. A e «, c   eent is prioritized highly enough to make it into an iteration (a sprint). (See also [5, c9] [22, c17].)

## 6.3. Scope Matching

Scope matching [22, c14] involves ensuring that the scope of requirements to architect, design and construct does not exceed any Requirements for typical software not only have wide breadth; they must also have significant depth. The tension created by simultaneous breadth-wise and depth-wise requirements in real-world projects often prompts teams to perform requirements activities iteratively. At some points, elicitation and analysis favor expanding the breadth of requirements knowledge, while at other points, expanding the depth is called for. In practice, it is highly unlikely that all requirements work can be done in a single pass through the subject matter. (See also [6, c2, c9].)

## 7.2. Requirements Prioritization [1*, c16]

Prioritizing requirements is useful throughout a software project because it helps focus software engineers on delivering the most valuable functionality soonest. It also helps support intelligent trade-off decisions involving conflict resolution and scope matching. Prioritized requirements also help in maintenance beyond spam filter. When considering happiness, the initial development project itself. Defects or satisfaction, from implementing features raised against higher-priority requirements combined with unhappiness (or dissatisfacshould probably be repaired before defects tion) from not implementing certain features, raised against lower-priority ones. developers would generally give handling attachments a higher priority than the effective spam filter.

The second key question is "How can we convert the set of relevant factors into an expression of priority?" The formula

<!-- formula-not-decoded -->

A variety of prioritization schemes are available. Answering a few key questions can help engineers choose the best approach. The first question is "What factors are relevant in determining the priority of one requirement over another?" The following factors might be relevant to a project:

· value; desirability; client, customer and is just one example of an objective function to user satisfaction; do so. The choice of measurement schemes for undesirability; client, customer and user the relevant factors can impose constraints dissatisfaction (Kano model, below); on the objective function. (See Measurement cost to deliver; Theory in Computing Foundations).

cost to maintain over the software's serOnce the priority of the requirements has vice life; been determined, those priorities must be technical risk of implementation; specified in a way that can be communicated · risk that users will not use it even if to all stakeholders. Several ways to do this are implemented. possible, including the following:

The Kano model, which underlies [6, c17] shows that considering only value, desirability or satisfaction can lead to erroneous priorities. A better understanding of priorities comes from considering how unhappy stakeholders would be if that requirement were not satisfied. For example, consider a project to develop an email client. Two candidate requirements might relate to:

## 1. Having an effective spam filter

## 2. Handling attachments on emails

Prioritization must weigh both the satisfaction users will experience from having certain features and the dissatisfaction they will experience if they lack certain features. For example, users are more likely to be happy with an effective spam filter than with the ability to handle attachments, so the spam filter would be given a higher priority based on the satisfaction criterion. On the other hand, the inability to handle attachments would make many users extremely unhappy — much more so than not having an effective

- enumerated scale (e.g., must have, should have, nice to have);
- numerical scale (e.g., 1 to 10);
- Lists that sort the requirements in decreasing priority order.

Effective requirement prioritization focuses on finding groups of requirements with sim1lrs is ri vat rets ri otis orous measurement scales or debating small differences.

<!-- formula-not-decoded -->

Requirements tracing can serve two potentially useful purposes. One is to serve as an accounting exercise that documents consistency between pairs of related project work products. An important question might be "For each identified software requirement, are there identified design elements intended to satisfy it?" If no identified design elements can be found, then either that requirement is not satisfied in that design or the design is correct and one or more stated requirements can be deleted. Similarly, "For each identified interest). At the same time, the latter may design element, are there identified require- be rendered obsolete by a change in governments that cause it to exist?" If no identified ment legislation. Finally, some requirements requirements can be found, then either that can be very unstable; they can change during design element is unnecessary or the stated the project — possibly more than once. It is requirements are incomplete. useful to assess the likelihood that a requirement will change in a given time. Identifying potentially volatile requirements helps the software engineer establish a design more tolerant of change, (e.g., [20]). (See also [9, c4].)

<!-- formula-not-decoded -->

As a practical matter, it may be useful to have some concept of the volume of the requirements for a particular software product. could be traced to the linked code. The This number is useful in evaluating the size affected software requirements, design ele- of a new development project or the size of ments and code units could also be traced to a change in requirements and in estimating their linked test cases for further impact anal- the cost of development or maintenance tasks -   n    (  )   o    , on" volume of work needed to incorporate that inator in other measurements. Functional size measurement (FSM) is a technique for evaluating the size of a body of functional requirements. Story points can also be considered a measure of requirements size.

The other purpose is to assist in impact analysis of a proposed requirement change. If a particular system requirement were to change, for example, that system requirement could be traced to its linked software requirements. Not all linked software requirements would need to change. But each software requirement that would be affected could be traced to its linked design elements. Again, not all linked design elements would need to change. But each design element affected change to the system requirement.

Software requirements can be traced back to source documentation such as system requirements, standards documents and other relevant specifications. Software requirements can also be traced forward to design elements and requirements-based test cases. Finally, software requirements can also be traced forward to sections in a user manual describing the implemented functionality. (See also [23].)

Additional information on size measurement and standards can be found in the Software Engineering Process KA.

Many quality indicators have been developed that can be used to relate the quality of software requirements specification to other project variables such as cost, acceptance, 7.4. Requirements Stability and Volatility performance, schedule and reproducibility. [2*, s4.6] Quality indicators for individual software requirements and a requirements specification document as a whole can be derived from the desirable properties discussed in Section vice life. Some requirements are less stable; 3.1, Basic Requirements Analysis, earlier in this KA.

<!-- formula-not-decoded -->

This topic concerns assessing the quality and improvement of the requirements process. Its purpose is to emphasize the key role of the requirements process in a software product's

Some requirements are very stable; they will probably never change over the software's serthey might change over the service life but might not change during the development project. For example, in a banking application, requirements for functions to calculate and credit interest to customers' accounts are likely to be more stable than requirements to support different tax-free accounts. The former reflects a banking domain's fundamental feature (that accounts can earn cost and timeliness and in customer satisfac- 8.2. Requirements Modeling Tools tion. Furthermore, it helps align the require[1*, c30p506] [2*, s12.3.3] ments process with quality standards and process improvement models for software and At a minimum, a requirements modeling tool systems. Process quality and improvement are supports visually creating, modifying and closely related to both the Software Quality publishing model-based requirements speciKA and Software Engineering Process KA, fications. Some tools extend that by also procomprising the following:

- requirements process coverage by process improvement standards and models;
- requirements process measures and benchmarking;
- improvement planning and implementation;
- security/CIA (confidentiality, integrity, and availability) improvement/planning and implementation.

## 8. Software Requirements Tools [1*, c30]

The more formally defined a requirements Tools that help software engineers deal with specification language is, the more likely it software requirements fall broadly into three is that functional test cases can be at least categories: requirements management tools, partially derived mechanically. For example, requirements modeling tools and functional converting BDD scenarios into test cases is test case generation tools, as discussed below. not difficult. Another example involves state models. Positive test cases can be derived 8.1. Requirements Management Tools for each defined transition in that kind of [1*, c30pp506-510] 1 model. Negative test cases can be derived from the state and event combinations that do not appear. (See Section 8.2, Testing Tools in the Testing KA, for more information.) A process for deriving test cases from UML requirements models can be found in [9, c12].

In the most general case, such tools can only generate test case inputs. Determining practice, many organizations have invested an expected result is not always possible, in tools. However, many more manage their additional business domain expertise might

Requirements management tools support various activities, including storing requirements attributes, tracing, document generation and change control. Indeed, tracing and change control might only be practical when supported by a tool. Because requirements management is fundamental to good requirements requirements in more ad hoc and generally be necessary. less satisfactory ways (e.g., spreadsheets). (See also [5, c8].)

viding static analysis (e.g., syntax correctness, completeness and consistency). Formal analysis requires tool support to be practicable for anything other than trivial systems, and tools generally fall into two categories: theorem provers or model checkers. In neither case can proof be fully automated, and the competence in formal reasoning needed to use the tools restricts the wider formal analysis. Some tools also dynamically execute a specification (simulation).

## 8.3. Functional Test Case Generation Tools

## MATRIX OFTOPICS VS. REFERENCE MATERIAL

|                                                                      | Wiegers 2013 [1*]   | Sommerville 2016 [2*]   | Tockey 2005 [3*]   | Wing 1990 [4*]   |
|----------------------------------------------------------------------|---------------------|-------------------------|--------------------|------------------|
| 1. Software Requirements Fundamentals                                |                     |                         |                    |                  |
| 1.1. Deinition of a Software Requirement                             | c1pp5-6             | c4p102                  |                    |                  |
| 1.2. Categories of Software Requirements                             | c1pp7-12            | s4.1                    |                    |                  |
| 1.3. Software Product Requirements and Software Project Requirements | c1pp14-15           |                         |                    |                  |
| 1.4. Functional Requirements                                         | c1p9                | s4.1.1                  |                    |                  |
| 1.5. Nonfunctional Requirements                                      | c1pp10-11           | s4.1.2                  |                    |                  |
| 1.6. Technology Constraints                                          |                     |                         |                    |                  |
| 1.7. Quality of Service Constraints                                  |                     |                         |                    |                  |
| 1.8. Why Categorize Requirements This Way?                           |                     |                         |                    |                  |
| 1.9. System Requirements and Software Requirements                   |                     |                         |                    |                  |
| 1.10. Derived Requirements                                           |                     |                         |                    |                  |
| 1.11. Software Requirements Activities                               | c1pp15-18           | s4.2                    |                    |                  |
| 2. Requirements Elicitation                                          |                     |                         |                    |                  |
| 2.1. Requirements Sources                                            | c6                  | s4.3                    |                    |                  |
| 2.2. Common Requirements Elicitation Techniques                      | c7                  | s4.3                    |                    |                  |
| 3. Requirements Analysis                                             |                     |                         |                    |                  |
| 3.1. Basic Requirements Analysis                                     | c8-9                |                         |                    |                  |
| 3.2. Economics of Quality of Service Constraints                     |                     |                         | c1-27              |                  |
| 3.3. Formal Analysis                                                 |                     | s12.3.2-12.3.3          |                    |                  |
| 3.4. Addressing Conflict in Requirements                             |                     |                         |                    |                  |
| 4. Requirements Specification                                        |                     |                         |                    |                  |
| 4.1. Unstructured Natural Language Requirements Specification        | c11                 | s4.4.1                  |                    |                  |
| 4.2. Structured Natural Language Requirements Specification          | c8                  | s4.4.2                  |                    |                  |
| 4.3. Acceptance Criteria-Based Requirements Specification            |                     | s3.2.3, s8.2            |                    |                  |
| 4.4. Model-Based Requirements Specification                          | c12                 | c5                      |                    | pp8-11           |
| 4.5. Additional Attributes of Requirements                           | c27pp462-463        |                         |                    |                  |
| 4.6. Incremental and Comprehensive Requirements Specification        |                     |                         |                    |                  |
| 5. Requirements Validation                                           |                     |                         |                    |                  |
| 5.1. Requirements Reviews                                            | c17pp332-342        | c4p130                  |                    |                  |
| 5.2. Simulation and Execution                                        |                     |                         |                    |                  |
| 5.3. Prototyping                                                     | c17p342             | c4p130                  |                    |                  |

| 6. Requirements Management Activities             |              |         |      |
|---------------------------------------------------|--------------|---------|------|
| 6.1. Requirements Scrubbing                       |              |         |      |
| 6.2. Requirements Change Control                  | c28          | s4.6    |      |
| 6.3. Scope Matching                               |              |         |      |
| 7. Practical Considerations                       |              |         |      |
| 7.1. Iterative Nature of the Requirements         | Process      |         | s4.2 |
| 7.2. Requirements Prioritization                  | c16          |         |      |
| 7.3. Requirements Tracing                         | c29          |         |      |
| 7.4. Requirements Stability and Volatility        |              | s4.6    |      |
| 7.5. Measuring Requirements                       | c19          |         |      |
| 7.6. Requirements Process Quality and Improvement | c31          |         |      |
| 8. Software Requirements Tools                    |              |         |      |
| 8.1. Requirements Management Tools                | c30pp506-510 |         |      |
| 8.2. Requirements Modeling Tools                  | c30p506      | s12.3.3 |      |
| 8.3. Functional Test Case Generation Tools        |              |         |      |

## FURTHER READINGS

IIBA, A Guide to the Business Analysis Body of Knowledge® (BABOK® Guide) v3 [30]

[1*], offering a comprehensive discussion of software requirements.

T. Gilb, Competitive Engineering: A Handbook for Systems Engineering, Requirements The BABOK Guide is the reference body Engineering, and Software Engineering Using

[] ss n nss s s s community and provides a comprehensive description of that discipline. While This book presents a unique perspective on broader than just requirements and just requirements, emphasizing requirements prefor software, a very large portion of the cision and completeness along with a strong BABOK Guide content is relevant to soft- business value-driven motivation. ware requirements.

K. Wiegers, Software Development Pearls: Lessons

P. LaPlante, Requirements Engineering for from Fifty Years of Software Experience [21]. Software and Systems [5].

This book is a compendium of important hnae s      o  acd  s  so offering a comprehensive discussion of soft- based on Dr. Wiegers' extensive real-world ware requirements. experience. Chapter 2 is specific to software requirements.

S. Robertson and J. Robertson, Mastering the Requirements Process: Getting Requirements R. Fisher and W. Ury, Getting to Yes [25]. Right [6].

This book is a classic reference on principled This book is another potential alternative to negotiation and conflict resolution that serves as one good basis for addressing inevitable conflict in software requirements when there are multiple stakeholders.

- Software Engineering Using Planguage, Oxford, UK: Elsevier ButterworthHeinemann, 2005.
- N. Ahmad, Effects ofElectronic Communication [8] E. Yourdon, Modern Structured Analysis, on the Elicitation of Tacit Knowledge in Englewood Cliffs, NJ: PrenticeInterview Techniques for Small Software Hall, 1989. Developments [29].

This doctoral thesis shows how using four different types of electronic communication tools to discuss interview agenda details with interviewees before conducting semi-structured interviews for requirements elicitation improved elicitation of tacit (hidden) knowledge.

## REFERENCES

- [1*] K. E. Wiegers and J. Beatty, Software Requirements, 3rd ed., Redmond, WA: Microsoft Press, 2013.
- [9] S. Tockey, How to Engineer Software, Hoboken, NJ: Wiley, 2019.
- [10] S. Ambler, Agile Modeling: Effective Practices for eXtreme Programming and the Unified Process, Hoboken, NJ: Wiley, 2002.
- [11] A. Cockburn, Writing Effective Use Cases, Upper Saddle River, NJ: Addison-Wesley, 2000.
- [12] L. Constantine and L. Lockwood, Software for Use, Reading, MA: Addison-Wesley, 2000.
- [2*] I. Sommerville, Software Engineering, 10th [13] J. Wood and D. Silver, Joint Application ed., New York: Addison-Wesley, 2016.
- [3*] S. Tockey, Return on Software: Maximizing the Return on Your Software Investment, Boston, MA: AddisonWesley, 2005.
- [4*] J. M. Wing, "A Specifier's Introduction to Formal Methods," Computer, vol. 23, no. 9, 1990, pp. 8, 10-23.
- [5] P. Laplante and M. Kassab, Requirements Engineering for Software and Systems, 4th ed., Boca Raton, FL: CRC Press, 2022.
- [6] S. Robertson and J. Robertson, Mastering the Requirements Process: Getting Requirements Right, Upper Saddle River, NJ: AddisonWesley, 2013.
- [7] T. Gilb, Competitive Engineering: A Handbook for Systems Engineering, Requirements Engineering, and
- Development, New York, NY: Wiley, 1995.
- [14] E. Gottesdiener, Requirements by Collaboration, Boston, MA: AddisonWesley, 2002.
- [15]J. Terninko, Step by Step QFD, 2nd ed., Boca Raton, FL: CRC Press, 1997.
- [16] G. Salvendy, Handbook of Human Factors, 4th ed., Hoboken, NJ: Wiley, 2012.
- [17] T. Brown and B. Katz, Change by Design: How Design Thinking Transforms Organizations and Inspires Innovation, Revised and updated ed., New York, NY: Harper Collins, 2019.
- [18] S. McMenamin and J. Palmer, Essential Systems Analysis, New York, NY: Yourdon Press, 1984.
- [19] J. Smart, BDD in Action:

- Behavior-Driven Development for the Whole Software Lifecycle, Shelter Island, NY: Manning Publications, 2015.
- [20] D. Weiss and C. Lai, Software ProductLine Engineering: A Family-Based Software Development Process, Reading, MA: Addison-Wesley, 1999.
- [21] K. Wiegers, Software Development Pearls: Lessons from Fifty Years of Software Experience, Boston, MA: Addison-Wesley Professional, 2021.
- [22] S. McConnell, Rapid Development, Redmond, WA: Microsoft Press, 1996.
- [26]ISO/IEC/IEEE 29148 "Systems and software engineering – Life cycle processes – Requirements engineering," International Standards Organization, 2018.
- [27] ISO/IEC 25010: "System and software engineering – Systems and software Quality Requirements and Evaluation (SQuaRE) – System and software quality models," International Standards Organization, 2011.
- [28]ISO/IEC/IEEE, "ISO/IEC/IEEE 24765:2017 Systems and Software Engineering — Vocabulary,' 2nd ed. 2017.
- [23] O. Gotel and C. W. Finkelstein, "An Analysis of the Requirements Traceability[ Problem," presented at the Proceedings of the 1st International Conference on Requirements Engineering, 1994.
- [24]INCOSE, Systems Engineering Handbook: A Guide for System Life Cycle Processes and Activities, 3.2.2 ed., San Diego, US: International Council on Systems Engineering, 2012.
- [25]R. Fisher and W. Ury, Getting to Yes, 3rd ed., New York, NY: Penguin, 2011.
- [29]N. Ahmad, Effects of Electronic Communication on the Elicitation of Tacit Knowledge in Interview Techniques for Small Software Developments, doctoral thesis, University of Huddersfield, 2021.
- [30] IIBA, A Guide to the Business Analysis Body of Knowledge® (BABOK® Guide) v3, International Institute of Business Analysis, Toronto, Ontario, Canada, 2015.